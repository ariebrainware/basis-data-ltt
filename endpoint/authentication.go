package endpoint

import (
	"fmt"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request payload",
			Err: err,
		})
		return
	}

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
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
	}

	// Check if user exists
	var User model.User
	err = db.Model(&User).Where("email = ? AND password = ?", req.Email, hashedPassword).First(&User).Error
	if err == gorm.ErrRecordNotFound {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "User not found, please sign up first",
			Err: fmt.Errorf("user not found"),
		})
		return
	}

	// Create JWT token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": User.Email,
		"exp":   time.Now().Add(time.Hour * 1).Unix(),
		"role":  User.RoleID,
	})

	tokenString, err := token.SignedString(util.JWTSecretByte)
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

	// Return the token in a JSON response
	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Login successful",
		Data: tokenString,
	})
}

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

	// Connect to the database
	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
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

	// Respond with a success message
	util.CallSuccessOK(c, util.APISuccessParams{
		Msg: "Logout successful",
	})
}

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Signup(c *gin.Context) {
	var req SignupRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request payload",
			Err: err,
		})
		return
	}

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
		})
		return
	}
	var existingUser *model.User
	err = db.First(&existingUser, "email = ?", req.Email).Error
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

	tokenString, err := token.SignedString(util.JWTSecretByte)
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
