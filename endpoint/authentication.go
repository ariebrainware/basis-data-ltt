package endpoint

import (
	"fmt"
	"time"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"gorm.io/gorm"
)

var jwtSecret = []byte("b54a241b0c864466e44ac93eed2d5719bfb887801f708f410db9a566eecee28122eb9df518b5116b5c19ded4519978ec7602116a7446ba4b057d4c8f220fef2df65e6c42c26854d9ff156c8d6561856530cec8ab5eeb2e011e259ae3c4ba4947201faf1573ae899078f6a220cc2c99e3d933fcfbb079b4b98345f873199e4f53b9b2781140e708b9ee2c213b443e371454ae479ace3a6b703de3de3dee7451463fb31a054bfb9f0fee73852dc09dc800ce37b6b79b83a0e280d414ed32444d3dd0674615803146e6ecca1ffffbfa8f8f0c8441ba5195911b78b7104ebd53223169c4073138c2e7101d5efb3119d981e7d839dd1c737da42bdd7220755ad60fbc")

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

	// Hash the provided password using HMAC-SHA256 with jwtSecret as key
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(req.Password))
	hashedPassword := hex.EncodeToString(h.Sum(nil))

	// Check if user exists
	var User model.User
	err = db.Model(&User).Where("email = ? AND password = ?", req.Email, hashedPassword).First(&model.User{}).Error
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
		"role":  "admin",
	})

	tokenString, err := token.SignedString(jwtSecret)
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

	// Here you would typically hash the password and store the user in your database.
	// This example omits database operations for simplicity.

	// Hash the password using HMAC-SHA256 with jwtSecret as key.
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(req.Password))
	hashedPassword := hex.EncodeToString(h.Sum(nil))

	newUser := model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		Role:     "admin",
	}

	// Insert the new user into the database.
	if err := db.Create(&newUser).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create new user",
			Err: err,
		})
		return
	}
	// Optionally generate a JWT token upon successful signup.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": req.Email,
		"exp":   time.Now().Add(time.Hour * 1).Unix(),
		"role":  newUser.Role,
	})

	tokenString, err := token.SignedString(jwtSecret)
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
