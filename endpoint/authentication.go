package endpoint

import (
	"context"
	"fmt"
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
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"password123"`
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

	if req.Password == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request payload",
			Err: fmt.Errorf("password cannot be empty"),
		})
		return
	}

	// Get client info for logging
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Check if user exists
	var user model.User
	err := db.Model(&user).Where("email = ?", req.Email).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		util.LogLoginFailure(req.Email, clientIP, userAgent, "user not found")
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid email or password",
			Err: fmt.Errorf("user not found"),
		})
		return
	}
	if err != nil {
		util.LogLoginFailure(req.Email, clientIP, userAgent, "database error")
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database error",
			Err: err,
		})
		return
	}

	// Check if account is locked
	if user.LockedUntil != nil && *user.LockedUntil > time.Now().Unix() {
		lockExpiry := time.Unix(*user.LockedUntil, 0)
		util.LogLoginFailure(req.Email, clientIP, userAgent, "account locked")
		util.CallUserError(c, util.APIErrorParams{
			Msg: fmt.Sprintf("Account is locked until %s due to multiple failed login attempts", lockExpiry.Format(time.RFC3339)),
			Err: fmt.Errorf("account locked"),
		})
		return
	}

	// Verify password (supports both Argon2id and legacy HMAC)
	passwordMatch, err := util.VerifyPassword(req.Password, user.Password, user.PasswordSalt)
	if err != nil {
		util.LogLoginFailure(req.Email, clientIP, userAgent, "password verification error")
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Password verification failed",
			Err: err,
		})
		return
	}

	if !passwordMatch {
		// Increment failed attempts
		user.FailedAttempts++
		
		// Lock account after 5 failed attempts
		if user.FailedAttempts >= 5 {
			lockUntil := time.Now().Add(15 * time.Minute).Unix()
			user.LockedUntil = &lockUntil
			util.LogAccountLocked(user.ID, user.Email, clientIP, "too many failed login attempts")
		}
		
		if err := db.Save(&user).Error; err != nil {
			util.LogLoginFailure(req.Email, clientIP, userAgent, "failed to update failed attempts")
		}
		
		util.LogLoginFailure(req.Email, clientIP, userAgent, "invalid password")
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid email or password",
			Err: fmt.Errorf("invalid password"),
		})
		return
	}

	// Reset failed attempts on successful login
	if user.FailedAttempts > 0 || user.LockedUntil != nil {
		user.FailedAttempts = 0
		user.LockedUntil = nil
		if err := db.Save(&user).Error; err != nil {
			// Log but don't fail the login
			util.LogSecurityEvent(util.SecurityEvent{
				EventType: util.EventSuspiciousActivity,
				UserID:    fmt.Sprintf("%d", user.ID),
				Email:     user.Email,
				IP:        clientIP,
				Message:   fmt.Sprintf("Failed to reset failed attempts: %v", err),
			})
		}
	}

	// Check role
	var role model.Role
	err = db.Model(&role).Where("id = ?", user.RoleID).First(&role).Error
	if err == gorm.ErrRecordNotFound {
		util.LogLoginFailure(req.Email, clientIP, userAgent, "role not found")
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Role not found",
			Err: fmt.Errorf("role not found"),
		})
		return
	}

	// Create JWT token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": user.Email,
		"exp":   time.Now().Add(time.Hour * 1).Unix(),
		"role":  user.RoleID,
	})

	tokenString, err := token.SignedString(util.GetJWTSecretByte())
	if err != nil {
		util.LogLoginFailure(req.Email, clientIP, userAgent, "token generation failed")
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Could not generate token",
			Err: err,
		})
		return
	}

	// Record Session
	session := model.Session{
		UserID:       user.ID,
		SessionToken: tokenString,
		ExpiresAt:    time.Now().Add(time.Hour * 1),
		ClientIP:     clientIP,
		Browser:      userAgent,
	}

	if err := db.Create(&session).Error; err != nil {
		util.LogLoginFailure(req.Email, clientIP, userAgent, "session creation failed")
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to record session",
			Err: err,
		})
		return
	}

	// Also store session in Redis for fast validation (key: session:<token> -> "userID:roleID")
	if rdb := config.GetRedisClient(); rdb != nil {
		exp := time.Until(session.ExpiresAt)
		val := fmt.Sprintf("%d:%d", session.UserID, role.ID)
		_ = rdb.Set(context.Background(), fmt.Sprintf("session:%s", tokenString), val, exp).Err()
		// Also update per-user set via util helper
		_ = util.AddSessionToUserSet(session.UserID, tokenString, exp)
	}

	// Log successful login
	util.LogLoginSuccess(user.ID, user.Email, clientIP, userAgent)

	// Return the token in a JSON response
	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Login successful",
		Data: LoginResponse{Token: tokenString, Role: role.Name, UserID: user.ID},
	})
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
	Name     string `json:"name" example:"John Doe"`
	Email    string `json:"email" example:"john@example.com"`
	Password string `json:"password" example:"password123"`
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

	// Validate password strength
	if len(req.Password) < 8 {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Password must be at least 8 characters long",
			Err: fmt.Errorf("password too short"),
		})
		return
	}

	var existingUser model.User
	err := db.First(&existingUser, "email = ?", req.Email).Error
	if err != gorm.ErrRecordNotFound {
		if err == nil {
			util.CallUserError(c, util.APIErrorParams{
				Msg: "Email already exists",
				Err: fmt.Errorf("email already exists"),
			})
			return
		}
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database error",
			Err: err,
		})
		return
	}

	// Generate salt and hash password using Argon2id
	salt, err := util.GenerateSalt()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to generate password salt",
			Err: err,
		})
		return
	}

	hashedPassword, err := util.HashPasswordArgon2(req.Password, salt)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to hash password",
			Err: err,
		})
		return
	}

	newUser := model.User{
		Name:         req.Name,
		Email:        req.Email,
		Password:     hashedPassword,
		PasswordSalt: salt,
		RoleID:       1,
		FailedAttempts: 0,
		LockedUntil:  nil,
	}

	// Insert the new user into the database.
	if err := db.Create(&newUser).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create new user",
			Err: err,
		})
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
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":   req.Email,
		"exp":     time.Now().Add(time.Hour * 1).Unix(),
		"role_id": newUser.RoleID,
	})

	tokenString, err := token.SignedString(util.GetJWTSecretByte())
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Could not generate token",
			Err: err,
		})
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
