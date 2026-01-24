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

	var hashedPassword string
	if req.Password != "" {
		hashedPassword = util.HashPassword(req.Password)
	} else {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request payload",
			Err: fmt.Errorf("password cannot be empty"),
		})
		return
	}

	// Check if user exists
	var User model.User
	err := db.Model(&User).Where("email = ? AND password = ?", req.Email, hashedPassword).First(&User).Error
	if err == gorm.ErrRecordNotFound {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "User not found, please sign up first",
			Err: fmt.Errorf("user not found"),
		})
		return
	}

	// Check role
	var role model.Role
	err = db.Model(&role).Where("id = ?", User.RoleID).First(&role).Error
	if err == gorm.ErrRecordNotFound {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Role not found",
			Err: fmt.Errorf("role not found"),
		})
		return
	}

	// Create JWT token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": User.Email,
		"exp":   time.Now().Add(time.Hour * 1).Unix(),
		"role":  User.RoleID,
	})

	tokenString, err := token.SignedString(util.GetJWTSecretByte())
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Could not generate token",
			Err: err,
		})
		return
	}

	// Record Session
	session := model.Session{
		UserID:       User.ID,
		SessionToken: tokenString,
		ExpiresAt:    time.Now().Add(time.Hour * 1),
		ClientIP:     c.ClientIP(),
		Browser:      c.Request.UserAgent(),
	}

	if err := db.Create(&session).Error; err != nil {
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

	// Return the token in a JSON response
	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Login successful",
		Data: LoginResponse{Token: tokenString, Role: role.Name, UserID: User.ID},
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

	var existingUser *model.User
	err := db.First(&existingUser, "email = ?", req.Email).Error
	if err == gorm.ErrRecordNotFound {
		fmt.Println(err)
	}

	if existingUser.Email == req.Email {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Email already exists",
			Err: fmt.Errorf("email already exists"),
		})
		return
	}

	// Hash the password using HMAC-SHA256 with jwtSecret as key.
	var hashedPassword string
	if req.Password != "" {
		hashedPassword = util.HashPassword(req.Password)
	}

	newUser := model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		RoleID:   1,
	}

	// Insert the new user into the database.
	if err := db.Create(&newUser).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create new user",
			Err: err,
		})
		return
	}

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

// VerifyPassword godoc
// @Summary      Verify current user's password
// @Description  Validate the provided current password for the authenticated user
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        password query string true "Current password"
// @Success      200 {object} util.APIResponse "Password verified"
// @Failure      400 {object} util.APIResponse "Invalid request payload"
// @Failure      401 {object} util.APIResponse "Invalid password or unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /verify-password [get]
func VerifyPassword(c *gin.Context) {
	// Read password from query parameter
	password := c.Query("password")
	if password == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Password is required",
			Err: fmt.Errorf("password query parameter is required"),
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
		util.CallUserError(c, util.APIErrorParams{
			Msg: "User not found",
			Err: err,
		})
		return
	}

	if user.Password == util.HashPassword(password) {
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
