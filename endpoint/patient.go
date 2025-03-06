package endpoint

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListPatients(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
		})
		return
	}

	var patients []model.Patient
	if err := db.Limit(limit).Offset(offset).Find(&patients).Error; err != nil {
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

type createPatientRequest struct {
	FullName      string   `json:"full_name"`
	Gender        string   `json:"gender"`
	Age           int      `json:"age"`
	Job           string   `json:"job"`
	Address       string   `json:"address"`
	PhoneNumber   []string `json:"phone_number"`
	HealthHistory []string `json:"health_history"`
	PatientCode   string   `json:"patient_code"`
}

func CreatePatient(c *gin.Context) {
	patientRequest := createPatientRequest{}

	err := c.ShouldBindJSON(&patientRequest)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}
	if patientRequest.FullName == "" || len(patientRequest.PhoneNumber) == 0 {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient payload is empty or missing required fields",
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

	var existingPatient model.Patient
	err = db.Transaction(func(tx *gorm.DB) error {
		// Check if username and phone already registered
		if err := tx.Where("full_name = ? AND (phone_number = ? OR phone_number IN ?)", patientRequest.FullName, strings.Join(patientRequest.PhoneNumber, ","), patientRequest.PhoneNumber).First(&existingPatient).Error; err == nil {
			return fmt.Errorf("patient already registered")
		}

		if err := tx.Create(&model.Patient{
			FullName:      patientRequest.FullName,
			Gender:        patientRequest.Gender,
			Age:           patientRequest.Age,
			Job:           patientRequest.Job,
			Address:       patientRequest.Address,
			PhoneNumber:   strings.Join(patientRequest.PhoneNumber, ","),
			PatientCode:   patientRequest.PatientCode,
			HealthHistory: strings.Join(patientRequest.HealthHistory, ","),
		}).Error; err != nil {
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
		Data: nil,
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

func GetPatientInfo(c *gin.Context) {
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

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient retrieved",
		Data: patient,
	})
}
