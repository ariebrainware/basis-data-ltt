package endpoint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseQueryParams(c *gin.Context) (int, int, string, string) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	keyword := c.Query("keyword")
	groupByDate := c.Query("group_by_date")
	return limit, offset, keyword, groupByDate
}

func applyGroupByDateFilter(query *gorm.DB, groupByDate string) *gorm.DB {
	switch groupByDate {
	case "last_2_days":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -2))
	case "last_3_months":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, -3, 0))
	case "last_6_months":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, -6, 0))
	}
	return query
}

func fetchPatients(limit, offset int, keyword, groupByDate string) ([]model.Patient, int64, error) {
	var patients []model.Patient
	var totalPatient int64

	db, err := config.ConnectMySQL()
	if err != nil {
		return nil, 0, err
	}

	query := db.Offset(offset).Order("patient_code ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if keyword != "" {
		query = query.Where("full_name LIKE ? OR patient_code LIKE ? OR address LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	query = applyGroupByDateFilter(query, groupByDate)

	if err := query.Find(&patients).Error; err != nil {
		return nil, 0, err
	}

	db.Model(&model.Patient{}).Count(&totalPatient)
	return patients, totalPatient, nil
}

func ListPatients(c *gin.Context) {
	limit, offset, keyword, groupByDate := parseQueryParams(c)

	patients, totalPatient, err := fetchPatients(limit, offset, keyword, groupByDate)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve patients",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patients retrieved",
		Data: map[string]interface{}{"total": totalPatient, "patients": patients},
	})
}

type createPatientRequest struct {
	FullName       string   `json:"full_name"`
	Gender         string   `json:"gender"`
	Age            int      `json:"age"`
	Job            string   `json:"job"`
	Address        string   `json:"address"`
	PhoneNumber    []string `json:"phone_number"`
	HealthHistory  []string `json:"health_history"`
	SurgeryHistory string   `json:"surgery_history"`
	PatientCode    string   `json:"patient_code"`
	Password       string   `json:"password,omitempty"`
	Email          string   `json:"email,omitempty"`
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
	var existingUser model.User
	err = db.Transaction(func(tx *gorm.DB) error {
		// Check if username and phone already registered
		if err := tx.Where("full_name = ? AND (phone_number = ? OR phone_number IN ?)", patientRequest.FullName, strings.Join(patientRequest.PhoneNumber, ","), patientRequest.PhoneNumber).First(&existingPatient).Error; err == nil {
			return fmt.Errorf("patient already registered")
		}

		if err := tx.Create(&model.Patient{
			FullName:       patientRequest.FullName,
			Gender:         patientRequest.Gender,
			Age:            patientRequest.Age,
			Job:            patientRequest.Job,
			Address:        patientRequest.Address,
			PhoneNumber:    strings.Join(patientRequest.PhoneNumber, ","),
			PatientCode:    patientRequest.PatientCode,
			HealthHistory:  strings.Join(patientRequest.HealthHistory, ","),
			SurgeryHistory: patientRequest.SurgeryHistory,
			Email:          patientRequest.Email,
			Password:       util.HashPassword(patientRequest.Password),
		}).Error; err != nil {
			return err
		}

		if patientRequest.Email != "" && patientRequest.Password != "" {
			// Check if users already registered
			if err := tx.Where("email = ?", patientRequest.Email).First(&existingUser).Error; err == nil {
				return fmt.Errorf("user already registered")
			}
			// Create user
			if err := tx.Create(&model.User{
				Name:     patientRequest.FullName,
				Email:    patientRequest.Email,
				Password: util.HashPassword(patientRequest.Password),
				RoleID:   2,
			}).Error; err != nil {
				return err
			}
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

func getPatientByID(c *gin.Context) (string, *gorm.DB, model.Patient, error) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing patient ID",
			Err: fmt.Errorf("patient ID is required"),
		})
		return "", nil, model.Patient{}, fmt.Errorf("patient ID is required")
	}

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
		})
		return "", nil, model.Patient{}, err
	}

	var patient model.Patient
	if err := db.First(&patient, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient not found",
			Err: err,
		})
		return "", nil, model.Patient{}, err
	}

	return id, db, patient, nil
}

func DeletePatient(c *gin.Context) {
	_, db, patient, err := getPatientByID(c)
	if err != nil {
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
	_, _, patient, err := getPatientByID(c)
	if err != nil {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient retrieved",
		Data: patient,
	})
}
