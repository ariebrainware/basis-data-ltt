package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createTherapistRequest struct {
	FullName    string `json:"full_name" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
	PhoneNumber string `json:"phone_number" binding:"required"`
	Address     string `json:"address" binding:"required"`
	DateOfBirth string `json:"date_of_birth" binding:"required"`
	NIK         string `json:"nik" binding:"required"`
	Weight      int    `json:"weight" binding:"required"`
	Height      int    `json:"height" binding:"required"`
	Role        string `json:"role" binding:"required"`
	IsApproved  bool   `json:"is_approved"`
}

func CreateTherapist(c *gin.Context) {
	therapistRequest := createTherapistRequest{}

	if err := c.ShouldBindJSON(&therapistRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}

	if err := validateTherapistRequest(therapistRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: err.Error(),
			Err: fmt.Errorf("invalid payload"),
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

	if err := createTherapistInDB(db, therapistRequest); err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create therapist",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Therapist created",
		Data: nil,
	})
}

func validateTherapistRequest(req createTherapistRequest) error {
	requiredFields := map[string]string{
		"FullName":    req.FullName,
		"PhoneNumber": req.PhoneNumber,
		"NIK":         req.NIK,
	}

	for fieldName, fieldValue := range requiredFields {
		if fieldValue == "" {
			return fmt.Errorf("%s is empty or missing required fields", fieldName)
		}
	}
	return nil
}

func createTherapistInDB(db *gorm.DB, req createTherapistRequest) error {
	var hashedPassword string
	if req.Password != "" {
		hashedPassword = util.HashPassword(req.Password)
	}

	var existingTherapist model.Therapist
	return db.Transaction(func(tx *gorm.DB) error {
		// Check if email and NIK already registered
		if err := tx.Where("email = ? AND NIK = ?").First(&existingTherapist).Error; err == nil {
			return fmt.Errorf("therapist already registered")
		}

		if err := tx.Create(&model.Therapist{
			FullName:    req.FullName,
			Email:       req.Email,
			Password:    hashedPassword,
			PhoneNumber: req.PhoneNumber,
			Address:     req.Address,
			DateOfBirth: req.DateOfBirth,
			NIK:         req.NIK,
			Weight:      req.Weight,
			Height:      req.Height,
			Role:        req.Role,
			IsApproved:  req.IsApproved,
		}).Error; err != nil {
			return err
		}

		return nil
	})
}
