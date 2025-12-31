package endpoint

import (
	"fmt"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func getTherapistIDFromSession(db *gorm.DB, sessionToken string) (uint, error) {
	if sessionToken == "" {
		return 0, fmt.Errorf("session token is empty")
	}

	var therapistID uint

	// Single query to join sessions, users, and therapists and fetch therapist ID
	// Includes validation: session not expired, not soft-deleted, and user has therapist role (role_id = 3)
	err := db.Table("sessions").
		Select("therapists.id").
		Joins("JOIN users ON users.id = sessions.user_id").
		Joins("JOIN therapists ON therapists.email = users.email").
		Where("sessions.session_token = ? AND sessions.expires_at > ? AND sessions.deleted_at IS NULL AND users.role_id = 3", sessionToken, time.Now()).
		Scan(&therapistID).Error
	if err != nil {
		return 0, fmt.Errorf("failed to resolve therapist from session: %w", err)
	}
	if therapistID == 0 {
		return 0, fmt.Errorf("therapist not found for session")
	}

	return therapistID, nil
}

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

	// Count total with the same filters (without limit/offset)
	countQuery := db.Table("treatments").
		Joins("LEFT JOIN patients ON patients.patient_code = treatments.patient_code").
		Where("patients.deleted_at IS NULL")
	if keyword != "" {
		countQuery = countQuery.Where("patients.full_name LIKE ? OR treatments.patient_code = ?", "%"+keyword+"%", keyword)
	}
	if therapistID != 0 {
		countQuery = countQuery.Where("treatments.therapist_id = ?", therapistID)
	}
	if groupByDate != "" {
		countQuery = countQuery.Where("treatments.treatment_date like ?", groupByDate+"%")
	}
	if err := countQuery.Count(&totalTreatments).Error; err != nil {
		return nil, 0, err
	}

	return treatments, totalTreatments, nil
}

// ListTreatments godoc
// @Summary      List all treatments
// @Description  Get a paginated list of treatments with optional filtering by therapist, keyword, and date
// @Tags         Treatment
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results"
// @Param        offset query int false "Offset for pagination"
// @Param        therapist_id query int false "Filter by therapist ID"
// @Param        keyword query string false "Search keyword for patient name or patient code"
// @Param        group_by_date query string false "Filter by specific date (YYYY-MM-DD format)"
// @Param        filter_by_therapist query boolean false "Filter by logged-in therapist"
// @Success      200 {object} util.APIResponse{data=object} "Treatments fetched successfully"
// @Failure      400 {object} util.APIResponse "Invalid request or session error"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /treatment [get]
func ListTreatments(c *gin.Context) {
	limit, offset, therapistID, keyword, groupByDate := parseQueryParams(c)
	filterByTherapist := c.Query("filter_by_therapist") == "true"

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	// If filter_by_therapist is true, get therapist ID from session
	if filterByTherapist {
		sessionToken := c.GetHeader("session-token")
		sessionTherapistID, err := getTherapistIDFromSession(db, sessionToken)
		if err != nil {
			// Provide more specific error messages based on the root cause
			switch {
			case strings.Contains(err.Error(), "session token is empty"):
				util.CallUserError(c, util.APIErrorParams{
					Msg: "Session token is missing in 'session-token' header",
					Err: err,
				})
			case strings.Contains(err.Error(), "session not found"):
				util.CallUserError(c, util.APIErrorParams{
					Msg: "Session not found or has expired",
					Err: err,
				})
			case strings.Contains(err.Error(), "user not found"):
				util.CallUserError(c, util.APIErrorParams{
					Msg: "User associated with the session was not found",
					Err: err,
				})
			case strings.Contains(err.Error(), "therapist not found"):
				util.CallUserError(c, util.APIErrorParams{
					Msg: "Therapist associated with the user was not found",
					Err: err,
				})
			default:
				util.CallServerError(c, util.APIErrorParams{
					Msg: "Failed to get therapist ID from session",
					Err: err,
				})
			}
			return
		}
		therapistID = int(sessionTherapistID)
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

// CreateTreatment godoc
// @Summary      Create a new treatment
// @Description  Add a new treatment record
// @Tags         Treatment
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body model.TreatementRequest true "Treatment information"
// @Success      200 {object} util.APIResponse "Treatment created successfully"
// @Failure      400 {object} util.APIResponse "Invalid request or duplicate treatment"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /treatment [post]
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

// UpdateTreatment godoc
// @Summary      Update treatment information
// @Description  Update an existing treatment record
// @Tags         Treatment
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Treatment ID"
// @Param        request body model.Treatment true "Updated treatment information"
// @Success      200 {object} util.APIResponse{data=model.Treatment} "Treatment updated successfully"
// @Failure      400 {object} util.APIResponse "Invalid request or treatment not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /treatment/{id} [patch]
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

// DeleteTreatment godoc
// @Summary      Delete a treatment
// @Description  Soft delete a treatment record by ID
// @Tags         Treatment
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Treatment ID"
// @Success      200 {object} util.APIResponse "Treatment deleted successfully"
// @Failure      400 {object} util.APIResponse "Treatment not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /treatment/{id} [delete]
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
