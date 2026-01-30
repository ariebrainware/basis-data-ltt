package endpoint

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"password123"`
}

type LoginResponse struct {
	Token  string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	Role   string `json:"role" example:"Admin"`
	UserID uint   `json:"user_id" example:"1"`
}

// Login godoc
// @Summary      User login
// @Description  Authenticate user with email and password
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body LoginRequest true "Login credentials"
// @Success      200 {object} util.APIResponse{data=LoginResponse} "Login successful"
// @Failure      400 {object} util.APIResponse "Invalid request payload"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /login [post]
func Login(c *gin.Context) {
	var req LoginRequest

	if !bindJSONOrRespond(c, &req, "Invalid request payload") {
		return
	}

	db, ok := getDBOrRespond(c)
	if !ok {
		return
	}

	if req.Password == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request payload",
			Err: fmt.Errorf("password cannot be empty"),
		})
		return
	}

	// Get client info for logging
	ci := clientInfo{IP: c.ClientIP(), Agent: c.Request.UserAgent()}
	ctx := loginContext{C: c, DB: db, Email: req.Email, CI: ci}

	// Load user
	user, ok := loadUserForLogin(ctx)
	if !ok {
		return
	}

	// Check lock
	if !ensureAccountNotLocked(ctx, &user) {
		return
	}

	// Verify password
	if !verifyPasswordOrRespond(ctx, &user, req.Password) {
		return
	}

	if !finalizeLogin(ctx, &user, req.Password) {
		return
	}
}

// helper types and functions to simplify Login flow
type clientInfo struct {
	IP    string
	Agent string
}

type loginContext struct {
	C     *gin.Context
	DB    *gorm.DB
	Email string
	CI    clientInfo
}

func bindJSONOrRespond(c *gin.Context, dst interface{}, msg string) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: msg, Err: err})
		return false
	}
	return true
}

func getDBOrRespond(c *gin.Context) (*gorm.DB, bool) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return nil, false
	}
	return db, true
}

func loadUserForLogin(ctx loginContext) (model.User, bool) {
	user, err := loadUserByEmail(ctx.DB, ctx.Email)
	if err == gorm.ErrRecordNotFound {
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "user not found")
		util.CallUserError(ctx.C, util.APIErrorParams{Msg: "Invalid email or password", Err: fmt.Errorf("user not found")})
		return model.User{}, false
	}
	if err != nil {
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "database error")
		util.CallServerError(ctx.C, util.APIErrorParams{Msg: "Database error", Err: err})
		return model.User{}, false
	}
	return user, true
}

func ensureAccountNotLocked(ctx loginContext, user *model.User) bool {
	if locked, expiry := isAccountLocked(user); locked {
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "account locked")
		util.CallUserError(ctx.C, util.APIErrorParams{Msg: fmt.Sprintf("Account is locked until %s due to multiple failed login attempts", expiry.Format(time.RFC3339)), Err: fmt.Errorf("account locked")})
		return false
	}
	return true
}

func verifyPasswordOrRespond(ctx loginContext, user *model.User, plain string) bool {
	match, err := util.VerifyPassword(plain, user.Password, user.PasswordSalt)
	if err != nil {
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "password verification error")
		util.CallServerError(ctx.C, util.APIErrorParams{Msg: "Password verification failed", Err: err})
		return false
	}
	if !match {
		incrementFailedAttempts(ctx.DB, user, ctx.CI)
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "invalid password")
		util.CallUserError(ctx.C, util.APIErrorParams{Msg: "Invalid email or password", Err: fmt.Errorf("invalid password")})
		return false
	}
	return true
}

func fetchRoleOrRespond(ctx loginContext, roleID uint32) (model.Role, bool) {
	role, err := fetchRole(ctx.DB, roleID)
	if err == gorm.ErrRecordNotFound {
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "role not found")
		util.CallUserError(ctx.C, util.APIErrorParams{Msg: "Role not found", Err: fmt.Errorf("role not found")})
		return model.Role{}, false
	}
	if err != nil {
		util.CallServerError(ctx.C, util.APIErrorParams{Msg: "Database error", Err: err})
		return model.Role{}, false
	}
	return role, true
}

func createTokenOrRespond(ctx loginContext, user model.User) (string, bool) {
	tokenString, err := createJWTToken(user)
	if err != nil {
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "token generation failed")
		util.CallServerError(ctx.C, util.APIErrorParams{Msg: "Could not generate token", Err: err})
		return "", false
	}
	return tokenString, true
}

func recordSessionOrRespond(ctx loginContext, info SessionInfo) (model.Session, bool) {
	session, err := recordSession(ctx.DB, info)
	if err != nil {
		util.LogLoginFailure(ctx.Email, ctx.CI.IP, ctx.CI.Agent, "session creation failed")
		util.CallServerError(ctx.C, util.APIErrorParams{Msg: "Failed to record session", Err: err})
		return model.Session{}, false
	}
	return session, true
}

func finalizeLogin(ctx loginContext, user *model.User, plain string) bool {
	// Reset failed attempts if needed
	if err := resetFailedAttempts(ctx.DB, user); err != nil {
		util.LogSecurityEvent(util.SecurityEvent{EventType: util.EventSuspiciousActivity, UserID: fmt.Sprintf("%d", user.ID), Email: user.Email, IP: ctx.CI.IP, Message: fmt.Sprintf("Failed to reset failed attempts: %v", err)})
	}

	// Upgrade legacy password if needed (best-effort)
	_ = upgradeLegacyPasswordIfNeeded(ctx.DB, user, plain, ctx.CI)

	// Fetch role
	role, ok := fetchRoleOrRespond(ctx, user.RoleID)
	if !ok {
		return false
	}

	// Create token
	tokenString, ok := createTokenOrRespond(ctx, *user)
	if !ok {
		return false
	}

	// Record session
	sessionInfo := SessionInfo{UserID: user.ID, Token: tokenString, Client: ctx.CI, Expires: time.Now().Add(time.Hour * 1)}
	session, ok := recordSessionOrRespond(ctx, sessionInfo)
	if !ok {
		return false
	}

	// Store session in Redis (best-effort)
	if rdb := config.GetRedisClient(); rdb != nil {
		exp := time.Until(session.ExpiresAt)
		val := fmt.Sprintf("%d:%d", session.UserID, role.ID)
		_ = rdb.Set(context.Background(), fmt.Sprintf("session:%s", tokenString), val, exp).Err()
		_ = util.AddSessionToUserSet(session.UserID, tokenString, exp)
	}

	util.LogLoginSuccess(user.ID, user.Email, ctx.CI.IP, ctx.CI.Agent)
	util.CallSuccessOK(ctx.C, util.APISuccessParams{Msg: "Login successful", Data: LoginResponse{Token: tokenString, Role: role.Name, UserID: user.ID}})
	return true
}

func ensureEmailAvailable(c *gin.Context, db *gorm.DB, email string) bool {
	var existingUser model.User
	err := db.First(&existingUser, "email = ?", email).Error
	if err != gorm.ErrRecordNotFound {
		if err == nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Email already exists", Err: fmt.Errorf("email already exists")})
			return false
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Database error", Err: err})
		return false
	}
	return true
}

func hashPasswordForSignup(c *gin.Context, plain string) (string, string, bool) {
	salt, err := util.GenerateSalt()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to generate password salt", Err: err})
		return "", "", false
	}
	hashedPassword, err := util.HashPasswordArgon2(plain, salt)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to hash password", Err: err})
		return "", "", false
	}
	return hashedPassword, salt, true
}

func createUserOrRespond(c *gin.Context, db *gorm.DB, user *model.User) bool {
	if err := db.Create(user).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to create new user", Err: err})
		return false
	}
	return true
}

func createSignupTokenOrRespond(c *gin.Context, email string, roleID uint32) (string, bool) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 1).Unix(),
		"role_id": roleID,
	})

	tokenString, err := token.SignedString(util.GetJWTSecretByte())
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Could not generate token", Err: err})
		return "", false
	}
	return tokenString, true
}

func loadUserByEmail(db *gorm.DB, email string) (model.User, error) {
	var user model.User
	err := db.Model(&user).Where("email = ?", email).First(&user).Error
	return user, err
}

func isAccountLocked(user *model.User) (bool, time.Time) {
	if user.LockedUntil != nil && *user.LockedUntil > time.Now().Unix() {
		return true, time.Unix(*user.LockedUntil, 0)
	}
	return false, time.Time{}
}

func incrementFailedAttempts(db *gorm.DB, user *model.User, ci clientInfo) {
	user.FailedAttempts++
	if user.FailedAttempts >= 5 {
		lockUntil := time.Now().Add(15 * time.Minute).Unix()
		user.LockedUntil = &lockUntil
		util.LogAccountLocked(user.ID, user.Email, ci.IP, "too many failed login attempts")
	}
	if err := db.Save(user).Error; err != nil {
		util.LogLoginFailure(user.Email, ci.IP, ci.Agent, "failed to update failed attempts")
	}
}

func resetFailedAttempts(db *gorm.DB, user *model.User) error {
	if user.FailedAttempts > 0 || user.LockedUntil != nil {
		user.FailedAttempts = 0
		user.LockedUntil = nil
		return db.Save(user).Error
	}
	return nil
}

func upgradeLegacyPasswordIfNeeded(db *gorm.DB, user *model.User, plain string, ci clientInfo) error {
	if strings.HasPrefix(user.Password, "argon2id$") {
		return nil
	}
	salt, err := util.GenerateSalt()
	if err != nil {
		return err
	}
	hashed, herr := util.HashPasswordArgon2(plain, salt)
	if herr != nil {
		return herr
	}
	user.Password = hashed
	user.PasswordSalt = salt
	if err := db.Save(user).Error; err != nil {
		util.LogSecurityEvent(util.SecurityEvent{EventType: util.EventSuspiciousActivity, UserID: fmt.Sprintf("%d", user.ID), Email: user.Email, IP: ci.IP, Message: fmt.Sprintf("Failed to upgrade password hash: %v", err)})
		return err
	}
	util.LogSecurityEvent(util.SecurityEvent{EventType: util.EventPasswordChanged, UserID: fmt.Sprintf("%d", user.ID), Email: user.Email, IP: ci.IP, Message: "Upgraded password hash to Argon2"})
	return nil
}

func fetchRole(db *gorm.DB, roleID uint32) (model.Role, error) {
	var role model.Role
	err := db.Model(&role).Where("id = ?", roleID).First(&role).Error
	return role, err
}

func createJWTToken(user model.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"email": user.Email, "exp": time.Now().Add(time.Hour * 1).Unix(), "role": user.RoleID})
	return token.SignedString(util.GetJWTSecretByte())
}

// (removed old recordSession with many args)

// SessionInfo groups parameters for creating a session to avoid long argument lists.
type SessionInfo struct {
	UserID  uint
	Token   string
	Client  clientInfo
	Expires time.Time
}

func recordSession(db *gorm.DB, info SessionInfo) (model.Session, error) {
	session := model.Session{UserID: info.UserID, SessionToken: info.Token, ExpiresAt: info.Expires, ClientIP: info.Client.IP, Browser: info.Client.Agent}
	err := db.Create(&session).Error
	return session, err
}

// Logout godoc
// @Summary      User logout
// @Description  Invalidate the user session token
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Success      200 {object} util.APIResponse "Logout successful"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      400 {object} util.APIResponse "Session not found"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /logout [delete]
func Logout(c *gin.Context) {
	// Extract the session-token from the request header
	sessionToken := c.GetHeader("session-token")
	if sessionToken == "" {
		util.CallUserNotAuthorized(c, util.APIErrorParams{
			Msg: "Session token not provided",
			Err: fmt.Errorf("session token not provided"),
		})
		c.Abort()
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	// Find the session record in the database based on sessionToken
	var session model.Session
	if err := db.Where("session_token = ?", sessionToken).First(&session).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Session not found",
			Err: err,
		})
		return
	}

	// Get user info for logging
	var user model.User
	if err := db.First(&user, session.UserID).Error; err == nil {
		util.LogLogout(user.ID, user.Email, c.ClientIP(), c.Request.UserAgent())
	}

	// Delete the session record from the database
	if err := db.Where("session_token = ?", sessionToken).Delete(&session).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete session",
			Err: err,
		})
		return
	}

	// Also delete session from Redis if available
	if rdb := config.GetRedisClient(); rdb != nil {
		_ = rdb.Del(context.Background(), fmt.Sprintf("session:%s", sessionToken)).Err()
		// Also remove token from the per-user set via util helper
		_ = util.RemoveSessionTokenFromUserSet(session.UserID, sessionToken)
	}

	// Respond with a success message
	util.CallSuccessOK(c, util.APISuccessParams{
		Msg: "Logout successful",
	})
}

type SignupRequest struct {
	Name     string `json:"name" binding:"required" example:"John Doe"`
	Email    string `json:"email" binding:"required,email" example:"john@example.com"`
	Password string `json:"password" binding:"required,min=8" example:"password123"`
}

// Signup godoc
// @Summary      User signup
// @Description  Register a new user account
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body SignupRequest true "Signup details"
// @Success      200 {object} util.APIResponse{data=string} "Signup successful"
// @Failure      400 {object} util.APIResponse "Invalid request or email already exists"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /signup [post]
func Signup(c *gin.Context) {
	var req SignupRequest

	if !bindJSONOrRespond(c, &req, "Invalid request payload") {
		return
	}

	db, ok := getDBOrRespond(c)
	if !ok {
		return
	}

	if !ensureEmailAvailable(c, db, req.Email) {
		return
	}

	hashedPassword, salt, ok := hashPasswordForSignup(c, req.Password)
	if !ok {
		return
	}

	newUser := model.User{
		Name:           req.Name,
		Email:          req.Email,
		Password:       hashedPassword,
		PasswordSalt:   salt,
		RoleID:         1,
		FailedAttempts: 0,
		LockedUntil:    nil,
	}

	// Insert the new user into the database.
	if !createUserOrRespond(c, db, &newUser) {
		return
	}

	// Log successful signup
	util.LogSecurityEvent(util.SecurityEvent{
		EventType: util.EventSignupSuccess,
		UserID:    fmt.Sprintf("%d", newUser.ID),
		Email:     newUser.Email,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Message:   "User signed up successfully",
	})

	// Generate a JWT token upon successful signup.
	tokenString, ok := createSignupTokenOrRespond(c, req.Email, newUser.RoleID)
	if !ok {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Signup successful",
		Data: tokenString,
	})
}

// VerifyPasswordRequest represents the request body for password verification
type VerifyPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// VerifyPassword godoc
// @Summary      Verify current user's password
// @Description  Validate the provided current password for the authenticated user
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body VerifyPasswordRequest true "Password to verify"
// @Success      200 {object} util.APIResponse "Password verified"
// @Failure      400 {object} util.APIResponse "Invalid request payload"
// @Failure      401 {object} util.APIResponse "Invalid password or unauthorized"
// @Failure      404 {object} util.APIResponse "User not found"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /verify-password [post]
func VerifyPassword(c *gin.Context) {
	// Read password from JSON body
	var req VerifyPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request payload",
			Err: err,
		})
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		util.CallUserNotAuthorized(c, util.APIErrorParams{
			Msg: "User not authenticated",
			Err: fmt.Errorf("user id not found in context"),
		})
		return
	}

	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{
				Msg: "User not found",
				Err: err,
			})
			return
		}
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve user",
			Err: err,
		})
		return
	}

	// Use constant-time comparison to prevent timing attacks
	passwordMatch, err := util.VerifyPassword(req.Password, user.Password, user.PasswordSalt)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Password verification failed",
			Err: err,
		})
		return
	}

	if passwordMatch {
		util.CallSuccessOK(c, util.APISuccessParams{
			Msg:  "Password verified",
			Data: map[string]bool{"verified": true},
		})
		return
	}

	util.CallUserNotAuthorized(c, util.APIErrorParams{
		Msg: "Invalid password",
		Err: fmt.Errorf("provided password does not match"),
	})
}
