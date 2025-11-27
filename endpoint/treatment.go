package endpoint

import (
	"strings"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

func fetchTreatments(limit, offset, therapistID int, keyword, groupByDate string) ([]model.ListTreatementResponse, int64, error) {
	db, err := config.ConnectMySQL()
	if err != nil {
		return nil, 0, err
	}

	var treatments []model.ListTreatementResponse
	var totalTreatments int64

	query := db.Table("treatments").
		Joins("LEFT JOIN therapists ON therapists.id = treatments.therapist_id").
		Joins("LEFT JOIN patients ON patients.patient_code = treatments.patient_code").
		Select("treatments.*, therapists.full_name as therapist_name, patients.full_name as patient_name, patients.age as age").
		Where("patients.deleted_at IS NULL")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if keyword != "" {
		query = query.Order("treatments.treatment_date DESC").Where("patients.full_name LIKE ? OR treatments.patient_code = ?", "%"+keyword+"%", keyword)

	} else {
		query = query.Order("treatments.created_at DESC")
	}
	if therapistID != 0 {
		query = query.Where("treatments.therapist_id = ?", therapistID)
	}
	if groupByDate != "" {
		query = query.Where("treatments.treatment_date like ?", groupByDate+"%")
	}

	if err := query.Find(&treatments).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Model(&model.Treatment{}).Count(&totalTreatments).Error; err != nil {
		return nil, 0, err
	}

	return treatments, totalTreatments, nil
}

func ListTreatments(c *gin.Context) {
	limit, offset, keyword, therapistID, groupByDate := parseQueryParams(c)

	treatments, totalTreatments, err := fetchTreatments(limit, offset, keyword, therapistID, groupByDate)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to fetch treatments",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Treatments fetched successfully",
		Data: map[string]interface{}{"total": totalTreatments, "treatments": treatments},
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
