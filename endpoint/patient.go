package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListPatients(c *gin.Context) {
	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
		})
		return
	}

	var patients []model.Patient
	if err := db.Find(&patients).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve patients",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patients retrieved",
		Data: patients,
	})
}

func CreatePatient(c *gin.Context) {
	patient := model.Patient{}

	if err := c.ShouldBindJSON(&patient); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}

	// validate the patient data here is no empty
	if patient.FullName == "" || patient.PhoneNumber == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Full name and phone number are required",
			Err: fmt.Errorf("full name and phone number are required"),
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

	err = db.Transaction(func(tx *gorm.DB) error {
		// Check if username and phone already registered
		var existingPatient model.Patient
		if err := tx.Where("full_name = ? AND phone_number = ?", patient.FullName, patient.PhoneNumber).First(&existingPatient).Error; err == nil {
			return fmt.Errorf("patient already registered")
		}

		// Create the patient record within the transaction.
		if err := tx.Create(&patient).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create patient",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient created",
		Data: patient,
	})
}

func UpdatePatient(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing patient ID",
			Err: fmt.Errorf("patient ID is required"),
		})
		return
	}

	patient := model.Patient{}
	if err := c.ShouldBindJSON(&patient); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
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

	var existingPatient model.Patient
	if err := db.First(&existingPatient, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient not found",
			Err: err,
		})
		return
	}

	if err := db.Model(&existingPatient).Updates(patient).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to update patient",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient updated",
		Data: existingPatient,
	})
}

func DeletePatient(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing patient ID",
			Err: fmt.Errorf("patient ID is required"),
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

	var patient model.Patient
	if err := db.First(&patient, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient not found",
			Err: err,
		})
		return
	}

	if err := db.Delete(&patient).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete patient",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg: "Patient deleted",
	})
}
