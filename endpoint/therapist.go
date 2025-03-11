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

	err := c.ShouldBindJSON(&therapistRequest)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}
	requiredFields := map[string]string{
		"FullName":    therapistRequest.FullName,
		"PhoneNumber": therapistRequest.PhoneNumber,
		"NIK":         therapistRequest.NIK,
	}

	for fieldName, fieldValue := range requiredFields {
		if fieldValue == "" {
			util.CallUserError(c, util.APIErrorParams{
				Msg: fmt.Sprintf("%s is empty or missing required fields", fieldName),
				Err: fmt.Errorf("invalid payload"),
			})
			return
		}
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
	if therapistRequest.Password != "" {
		hashedPassword = util.HashPassword(therapistRequest.Password)
	}

	var existingTherapist model.Therapist
	err = db.Transaction(func(tx *gorm.DB) error {
		// Check if email and NIK already registered
		if err := tx.Where("email = ? AND NIK = ?").First(&existingTherapist).Error; err == nil {
			return fmt.Errorf("therapist already registered")
		}

		if err := tx.Create(&model.Therapist{
			FullName:    therapistRequest.FullName,
			Email:       therapistRequest.Email,
			Password:    hashedPassword,
			PhoneNumber: therapistRequest.PhoneNumber,
			Address:     therapistRequest.Address,
			DateOfBirth: therapistRequest.DateOfBirth,
			NIK:         therapistRequest.NIK,
			Weight:      therapistRequest.Weight,
			Height:      therapistRequest.Height,
			Role:        therapistRequest.Role,
			IsApproved:  therapistRequest.IsApproved,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
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
