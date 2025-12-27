package endpoint

import (
	"fmt"
	"strings"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func fetchTreatments(db *gorm.DB, limit, offset, therapistID int, keyword, groupByDate string) ([]model.ListTreatementResponse, int64, error) {
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
	limit, offset, therapistID, keyword, groupByDate := parseQueryParams(c)

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	treatments, totalTreatments, err := fetchTreatments(db, limit, offset, therapistID, keyword, groupByDate)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to fetch treatments",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Treatments fetched successfully",
		Data: map[string]interface{}{"total": totalTreatments, "total_fetched": len(treatments), "treatments": treatments},
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

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	var existingTreatment model.Treatment
	if err := db.Where("treatment_date = ? AND patient_code = ?", createTreatmentRequest.TreatmentDate, createTreatmentRequest.PatientCode).First(&existingTreatment).Error; err == nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Treatment with this date already exists for this patient",
			Err: fmt.Errorf("duplicate treatment date"),
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

func UpdateTreatment(c *gin.Context) {
	treatmentID := c.Param("id")
	if treatmentID == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing treatment ID",
			Err: fmt.Errorf("treatment ID is required"),
		})
		return
	}

	treatment := model.Treatment{}
	if err := c.ShouldBindJSON(&treatment); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid input data",
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

	var existingTreatment model.Treatment
	if err := db.First(&existingTreatment, treatmentID).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Treatment not found",
			Err: err,
		})
		return
	}

	if err := db.Model(&existingTreatment).Updates(treatment).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to update treatment",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Treatment updated successfully",
		Data: existingTreatment,
	})
}

func DeleteTreatment(c *gin.Context) {
	treatmentID := c.Param("id")
	if treatmentID == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing treatment ID",
			Err: fmt.Errorf("treatment ID is required"),
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

	var existingTreatment model.Treatment
	if err := db.First(&existingTreatment, treatmentID).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Treatment not found",
			Err: err,
		})
		return
	}

	if err := db.Delete(&existingTreatment).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete treatment",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Treatment deleted successfully",
		Data: nil,
	})
}
