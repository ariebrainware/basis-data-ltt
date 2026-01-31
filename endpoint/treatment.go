package endpoint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// treatmentQueryParams encapsulates all query parameters for treatment listing
type treatmentQueryParams struct {
	limit       int
	offset      int
	therapistID int
	keyword     string
	groupByDate string
	jakartaLoc  *time.Location
}

// Common validation helpers
func getDBOrAbort(c *gin.Context) (*gorm.DB, bool) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return nil, false
	}
	return db, true
}

func validateTreatmentID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing treatment ID",
			Err: fmt.Errorf("treatment ID is required"),
		})
		return "", false
	}
	return id, true
}

func findTreatmentOrAbort(c *gin.Context, db *gorm.DB, treatmentID string) (*model.Treatment, bool) {
	var treatment model.Treatment
	if err := db.First(&treatment, treatmentID).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Treatment not found",
			Err: err,
		})
		return nil, false
	}
	return &treatment, true
}

// applyCreatedAtFilterForTreatments applies a created_at filter for supported ranges on treatment_date.
// Supported values: "last_2_days", "last_3_months", "last_6_months".
func applyCreatedAtFilterForTreatments(query *gorm.DB, groupByDate string) *gorm.DB {
	switch groupByDate {
	case "last_2_days":
		query = query.Where("treatments.treatment_date >= ?", time.Now().AddDate(0, 0, -2))
	case "last_3_months":
		query = query.Where("treatments.treatment_date >= ?", time.Now().AddDate(0, -3, 0))
	case "last_6_months":
		query = query.Where("treatments.treatment_date >= ?", time.Now().AddDate(0, -6, 0))
	}
	return query
}

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

func buildTreatmentBaseQuery(db *gorm.DB) *gorm.DB {
	return db.Table("treatments").
		Joins("LEFT JOIN therapists ON therapists.id = treatments.therapist_id").
		Joins("LEFT JOIN patients ON patients.patient_code = treatments.patient_code").
		Select("treatments.*, therapists.full_name as therapist_name, patients.full_name as patient_name, patients.age as age").
		Where("patients.deleted_at IS NULL")
}

func buildCountQuery(db *gorm.DB) *gorm.DB {
	return db.Table("treatments").
		Joins("LEFT JOIN patients ON patients.patient_code = treatments.patient_code").
		Where("patients.deleted_at IS NULL")
}

func applyPagination(query *gorm.DB, limit, offset int) *gorm.DB {
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	return query
}

func applyKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	if keyword != "" {
		kw := "%" + keyword + "%"
		return query.Order("treatments.treatment_date DESC").Where("patients.full_name LIKE ? OR treatments.patient_code = ?", kw, keyword)
	}
	return query.Order("treatments.created_at DESC")
}

func applyTherapistFilter(query *gorm.DB, therapistID int) *gorm.DB {
	if therapistID != 0 {
		return query.Where("treatments.therapist_id = ?", therapistID)
	}
	return query
}

func applyDateFilter(query *gorm.DB, groupByDate string, jakartaLoc *time.Location) *gorm.DB {
	if groupByDate == "" {
		return query
	}

	// Try parsing as explicit date first
	if start, err := time.ParseInLocation("2006-01-02", groupByDate, jakartaLoc); err == nil {
		end := start.Add(24 * time.Hour)
		return query.Where("treatments.treatment_date >= ? AND treatments.treatment_date < ?", start, end)
	}

	// Otherwise check predefined ranges
	validDates := map[string]bool{
		"last_2_days":   true,
		"last_3_months": true,
		"last_6_months": true,
	}
	if validDates[groupByDate] {
		return applyCreatedAtFilterForTreatments(query, groupByDate)
	}
	return query
}

func fetchTreatments(db *gorm.DB, params treatmentQueryParams) ([]model.ListTreatementResponse, int64, error) {
	var treatments []model.ListTreatementResponse
	var totalTreatments int64

	// Build and execute main query
	query := buildTreatmentBaseQuery(db)
	query = applyPagination(query, params.limit, params.offset)
	query = applyKeywordFilter(query, params.keyword)
	query = applyTherapistFilter(query, params.therapistID)
	query = applyDateFilter(query, params.groupByDate, params.jakartaLoc)

	if err := query.Find(&treatments).Error; err != nil {
		return nil, 0, err
	}

	// Build and execute count query (same filters, no pagination)
	countQuery := buildCountQuery(db)
	countQuery = applyKeywordFilter(countQuery, params.keyword)
	countQuery = applyTherapistFilter(countQuery, params.therapistID)
	countQuery = applyDateFilter(countQuery, params.groupByDate, params.jakartaLoc)

	if err := countQuery.Count(&totalTreatments).Error; err != nil {
		return nil, 0, err
	}

	return treatments, totalTreatments, nil
}

func parseQueryInt(c *gin.Context, key string, defaultVal int) int {
	if s := c.Query(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 0 {
			return v
		}
	}
	return defaultVal
}

func resolveTherapistIDFromSession(c *gin.Context, db *gorm.DB) (int, error) {
	sessionToken := c.GetHeader("session-token")
	therapistID, err := getTherapistIDFromSession(db, sessionToken)
	if err != nil {
		return 0, err
	}

	maxUint := ^uint(0) >> 1
	if therapistID > maxUint {
		return 0, fmt.Errorf("therapist id overflow: %d", therapistID)
	}
	return int(therapistID), nil
}

func handleSessionError(c *gin.Context, err error) {
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
	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	jakartaLoc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to load timezone",
			Err: err,
		})
		return
	}

	params := treatmentQueryParams{
		limit:       parseQueryInt(c, "limit", 0),
		offset:      parseQueryInt(c, "offset", 0),
		therapistID: parseQueryInt(c, "therapist_id", 0),
		keyword:     c.Query("keyword"),
		groupByDate: c.Query("group_by_date"),
		jakartaLoc:  jakartaLoc,
	}

	if c.Query("filter_by_therapist") == "true" {
		therapistID, err := resolveTherapistIDFromSession(c, db)
		if err != nil {
			handleSessionError(c, err)
			return
		}
		params.therapistID = therapistID
	}

	treatments, totalTreatments, err := fetchTreatments(db, params)
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

func checkDuplicateTreatment(c *gin.Context, db *gorm.DB, date string, patientCode string) bool {
	var existingTreatment model.Treatment
	if err := db.Where("treatment_date = ? AND patient_code = ?", date, patientCode).First(&existingTreatment).Error; err == nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Treatment with this date already exists for this patient",
			Err: fmt.Errorf("duplicate treatment date"),
		})
		return false
	}
	return true
}

func createTreatmentRecord(db *gorm.DB, req model.TreatementRequest) error {
	return db.Create(&model.Treatment{
		TreatmentDate: req.TreatmentDate,
		PatientCode:   req.PatientCode,
		TherapistID:   req.TherapistID,
		Issues:        req.Issues,
		Treatment:     strings.Join(req.Treatment, ","),
		Remarks:       req.Remarks,
		NextVisit:     req.NextVisit,
	}).Error
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
	var req model.TreatementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid input data",
			Err: err,
		})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	if !checkDuplicateTreatment(c, db, req.TreatmentDate, req.PatientCode) {
		return
	}

	if err := createTreatmentRecord(db, req); err != nil {
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
	treatmentID, ok := validateTreatmentID(c)
	if !ok {
		return
	}

	var updates model.Treatment
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid input data",
			Err: err,
		})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	existingTreatment, ok := findTreatmentOrAbort(c, db, treatmentID)
	if !ok {
		return
	}

	if err := db.Model(existingTreatment).Updates(updates).Error; err != nil {
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
	treatmentID, ok := validateTreatmentID(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	existingTreatment, ok := findTreatmentOrAbort(c, db, treatmentID)
	if !ok {
		return
	}

	if err := db.Delete(existingTreatment).Error; err != nil {
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
