package endpoint

import (
	"strings"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

func ListTreatments(c *gin.Context) {
	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to database",
			Err: err,
		})
		return
	}

	var data []model.ListTreatementResponse
	err = db.Table("treatments").
		Joins("LEFT JOIN therapists ON therapists.id = treatments.therapist_id").
		Joins("LEFT JOIN patients ON patients.patient_code = treatments.patient_code").
		Select("treatments.*, therapists.full_name as therapist_name, patients.full_name as patient_name").
		Scan(&data).Error
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to fetch treatments with join",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Treatments fetched successfully",
		Data: data,
	})
}

func CreateTreatment(c *gin.Context) {
	createTreatmentRequest := model.TreatementRequest{}
	if err := c.ShouldBindJSON(&createTreatmentRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid input data",
			Err: err,
		})
		return
	}

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to database",
			Err: err,
		})
		return
	}

	if err := db.Create(&model.Treatment{
		TreatmentDate: createTreatmentRequest.TreatmentDate,
		PatientCode:   createTreatmentRequest.PatientCode,
		TherapistID:   createTreatmentRequest.TherapistID,
		Issues:        createTreatmentRequest.Issues,
		Treatment:     strings.Join(createTreatmentRequest.Treatment, ","),
		Remarks:       createTreatmentRequest.Remarks,
		NextVisit:     createTreatmentRequest.NextVisit,
	}).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create treatment",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Treatment created successfully",
		Data: nil,
	})
}
