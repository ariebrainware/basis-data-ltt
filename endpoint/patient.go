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

type listQuery struct {
	Limit       int
	Offset      int
	Keyword     string
	GroupByDate string
	SortBy      string
	SortDir     string
}

func parseQueryParams(c *gin.Context) listQuery {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	keyword := c.Query("keyword")
	groupByDate := c.Query("group_by_date")
	sortBy := c.Query("sort")                       // supported values: full_name, patient_code
	sortDir := strings.ToLower(c.Query("sort_dir")) // supported values: asc, desc
	return listQuery{
		Limit:       limit,
		Offset:      offset,
		Keyword:     keyword,
		GroupByDate: groupByDate,
		SortBy:      sortBy,
		SortDir:     sortDir,
	}
}

// applyCreatedAtFilter applies a created_at filter for supported ranges.
// Supported values for groupByDate: "last_2_days", "last_3_months", "last_6_months".
func applyCreatedAtFilter(query *gorm.DB, groupByDate string) *gorm.DB {
	switch groupByDate {
	case "last_2_days":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -2))
	case "last_3_months":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, -3, 0))
	case "last_6_months":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, -6, 0))
	default:
		// If an unknown non-empty value is provided, log it for debugging.
		if groupByDate != "" {
			fmt.Printf("applyCreatedAtFilter: unknown group_by_date value: %s\n", groupByDate)
		}
	}
	return query
}

// getDBOrAbort retrieves the database connection or aborts with an error response.
// Returns the database connection and true if successful, or nil and false if failed.
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

func fetchPatients(db *gorm.DB, q listQuery) ([]model.Patient, int64, error) {
	var patients []model.Patient
	var totalPatient int64
	query := db

	// Determine order direction safely (only allow asc/desc)
	orderDir := "ASC"
	if strings.ToLower(q.SortDir) == "desc" {
		orderDir = "DESC"
	}

	// Apply sorting: if front-end requests sorting, use that; otherwise default to created_at DESC
	switch q.SortBy {
	case "full_name":
		query = query.Order(fmt.Sprintf("patients.full_name %s", orderDir))
	case "patient_code":
		query = query.Order(fmt.Sprintf("patients.patient_code %s", orderDir))
	default:
		query = query.Order("patients.created_at DESC")
	}

	if q.Limit > 0 {
		query = query.Limit(q.Limit)
	}
	if q.Offset > 0 {
		query = query.Offset(q.Offset)
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		query = query.Where("full_name LIKE ? OR patient_code LIKE ? OR address LIKE ? OR phone_number LIKE ?", kw, kw, kw, kw)
	}
	query = applyCreatedAtFilter(query, q.GroupByDate)

	if err := query.Find(&patients).Error; err != nil {
		return nil, 0, err
	}

	db.Model(&model.Patient{}).Count(&totalPatient)
	return patients, totalPatient, nil
}

// ListPatients godoc
// @Summary      List all patients
// @Description  Get a paginated list of patients with optional filtering
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results"
// @Param        offset query int false "Offset for pagination"
// @Param        keyword query string false "Search keyword for patient name, code, address, or phone"
// @Param        group_by_date query string false "Filter by date range (last_2_days, last_3_months, last_6_months)"
// @Param        sort query string false "Optional sort field: full_name|patient_code"
// @Param        sort_dir query string false "Optional sort direction: asc|desc"
// @Success      200 {object} util.APIResponse{data=object} "Patients retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient [get]
func ListPatients(c *gin.Context) {
	query := parseQueryParams(c)

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	patients, totalPatient, err := fetchPatients(db, query)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve patients",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patients retrieved",
		Data: map[string]interface{}{"total": totalPatient, "total_fetched": len(patients), "patients": patients},
	})
}

type createPatientRequest struct {
	FullName       string   `json:"full_name" example:"John Doe"`
	Gender         string   `json:"gender" example:"Male"`
	Age            int      `json:"age" example:"30"`
	Job            string   `json:"job" example:"Engineer"`
	Address        string   `json:"address" example:"123 Main St"`
	PhoneNumber    []string `json:"phone_number" example:"081234567890,081234567891"`
	HealthHistory  []string `json:"health_history" example:"Diabetes,Hypertension"`
	SurgeryHistory string   `json:"surgery_history" example:"Appendectomy 2020"`
	PatientCode    string   `json:"patient_code" example:"J001"`
	Password       string   `json:"password,omitempty" example:"password123"`
	Email          string   `json:"email,omitempty" example:"john@example.com"`
}

func normalizePhoneNumbers(numbers []string) []string {
	result := make([]string, 0, len(numbers))
	seen := make(map[string]struct{}, len(numbers))
	for _, n := range numbers {
		trimmed := strings.TrimSpace(n)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func hasDuplicatePatientByNameAndPhone(db *gorm.DB, fullName string, phoneNumbers []string) (bool, error) {
	if len(phoneNumbers) == 0 {
		return false, nil
	}
	phoneSet := make(map[string]struct{}, len(phoneNumbers))
	for _, p := range phoneNumbers {
		phoneSet[p] = struct{}{}
	}

	var matches []model.Patient
	if err := db.Where("full_name = ?", fullName).Find(&matches).Error; err != nil {
		return false, err
	}

	for _, m := range matches {
		stored := strings.Split(m.PhoneNumber, ",")
		for _, sp := range stored {
			if _, ok := phoneSet[strings.TrimSpace(sp)]; ok {
				return true, nil
			}
		}
	}

	return false, nil
}

func buildPatientCode(tx *gorm.DB, fullName, requestedCode string) (string, error) {
	if requestedCode != "" {
		return requestedCode, nil
	}

	initials := getInitials(fullName)
	var patientCodeTable model.PatientCode
	if err := tx.Order("id DESC").Where("alphabet = ?", initials).First(&patientCodeTable).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("patient code not found")
		}
		return "", err
	}

	newNumber := patientCodeTable.Number + 1
	patientCode := fmt.Sprintf("%s%d", initials, newNumber)
	if err := tx.Where("alphabet = ?", initials).Updates(&model.PatientCode{
		Number:   newNumber,
		Alphabet: initials,
		Code:     patientCode,
	}).Error; err != nil {
		return "", err
	}

	return patientCode, nil
}

func ensurePatientCodeAvailable(tx *gorm.DB, patientCode string) error {
	var existing model.Patient
	if err := tx.Where("patient_code = ?", patientCode).First(&existing).Error; err == nil {
		return fmt.Errorf("patient_code already registered")
	} else if err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}

func maybeCreateUser(tx *gorm.DB, req createPatientRequest) error {
	if !shouldCreateUser(req) {
		return nil
	}

	var existingUser model.User
	if err := tx.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return fmt.Errorf("email already registered")
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	return tx.Create(&model.User{
		Name:     req.FullName,
		Email:    req.Email,
		Password: util.HashPassword(req.Password),
		RoleID:   2,
	}).Error
}

// shouldCreateUser determines whether a user record should be created for the patient.
func shouldCreateUser(req createPatientRequest) bool {
	if req.Email == "" {
		return false
	}
	if req.Email == "-" {
		return false
	}
	if req.Password == "" {
		return false
	}
	return true
}

func buildPatientModel(req createPatientRequest, patientCode string, phoneNumbers []string) model.Patient {
	return model.Patient{
		FullName:       req.FullName,
		Gender:         req.Gender,
		Age:            req.Age,
		Job:            req.Job,
		Address:        req.Address,
		PhoneNumber:    strings.Join(phoneNumbers, ","),
		PatientCode:    patientCode,
		HealthHistory:  strings.Join(req.HealthHistory, ","),
		SurgeryHistory: req.SurgeryHistory,
		Email:          req.Email,
		Password:       util.HashPassword(req.Password),
	}
}

// CreatePatient godoc
// @Summary      Create a new patient
// @Description  Register a new patient (public endpoint - no authentication required)
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Param        request body createPatientRequest true "Patient information"
// @Success      200 {object} util.APIResponse "Patient created"
// @Failure      400 {object} util.APIResponse "Invalid request or patient already exists"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient [post]
func CreatePatient(c *gin.Context) {
	patientRequest := createPatientRequest{}

	if err := c.ShouldBindJSON(&patientRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}

	// Validate and normalize inputs
	normalizedPhones, err := prepareCreatePatient(&patientRequest)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient payload is empty or missing required fields",
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

	// Preliminary duplicate check before transaction to fail fast
	duplicate, err := hasDuplicatePatientByNameAndPhone(db, patientRequest.FullName, normalizedPhones)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to check existing patient",
			Err: err,
		})
		return
	}
	if duplicate {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient already exists with same name and phone number",
			Err: fmt.Errorf("patient duplicate detected"),
		})
		return
	}

	// Perform creation inside a transaction (extracted)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return createPatientInTx(tx, patientRequest, normalizedPhones)
	}); err != nil {
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

// prepareCreatePatient validates and normalizes the incoming patient request.
// Returns normalized phone numbers or an error when payload is invalid.
func prepareCreatePatient(req *createPatientRequest) ([]string, error) {
	req.FullName = util.NormalizeName(req.FullName)
	normalizedPhones := normalizePhoneNumbers(req.PhoneNumber)
	if req.FullName == "" || len(normalizedPhones) == 0 {
		return nil, fmt.Errorf("invalid payload")
	}
	return normalizedPhones, nil
}

// createPatientInTx performs the DB operations inside a transaction.
func createPatientInTx(tx *gorm.DB, req createPatientRequest, normalizedPhones []string) error {
	// Re-check for duplicate patient inside the transaction to avoid race conditions.
	duplicate, err := hasDuplicatePatientByNameAndPhone(tx, req.FullName, normalizedPhones)
	if err != nil {
		return err
	}
	if duplicate {
		return fmt.Errorf("patient already exists with same name and phone number")
	}

	patientCode, err := buildPatientCode(tx, req.FullName, req.PatientCode)
	if err != nil {
		return err
	}

	if err := ensurePatientCodeAvailable(tx, patientCode); err != nil {
		return err
	}

	if err := maybeCreateUser(tx, req); err != nil {
		return err
	}

	patient := buildPatientModel(req, patientCode, normalizedPhones)
	if err := tx.Create(&patient).Error; err != nil {
		return err
	}
	return nil
}

// UpdatePatient godoc
// @Summary      Update patient information
// @Description  Update an existing patient's information
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Patient ID"
// @Param        request body model.UpdatePatientRequest true "Updated patient information"
// @Success      200 {object} util.APIResponse{data=model.Patient} "Patient updated"
// @Failure      400 {object} util.APIResponse "Invalid request or patient not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient/{id} [patch]
func UpdatePatient(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing patient ID",
			Err: fmt.Errorf("patient ID is required"),
		})
		return
	}

	req := model.UpdatePatientRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
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

	_, existingPatient, err := getPatientByID(c, db)
	if err != nil {
		return
	}

	mergeUpdatePatient(&existingPatient, req)

	if err := db.Save(&existingPatient).Error; err != nil {
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

// mergeUpdatePatient merges non-zero/empty fields from req into existing.
func mergeUpdatePatient(existing *model.Patient, req model.UpdatePatientRequest) {
	updatePatientPhones(existing, req.PhoneNumber)
	updatePatientBasic(existing, req)
	updatePatientPassword(existing, req.Password)
}

// updatePatientPhones normalizes and merges phone numbers into existing patient
func updatePatientPhones(existing *model.Patient, phones []string) {
	if len(phones) == 0 {
		return
	}
	normalized := normalizePhoneNumbers(phones)
	if len(normalized) > 0 {
		existing.PhoneNumber = strings.Join(normalized, ",")
	}
}

// updatePatientBasic updates non-sensitive basic fields
func updatePatientBasic(existing *model.Patient, req model.UpdatePatientRequest) {
	updatePatientIdentity(existing, req)
	updatePatientDetails(existing, req)
}

// updatePatientIdentity updates identity-like fields
func updatePatientIdentity(existing *model.Patient, req model.UpdatePatientRequest) {
	if req.FullName != "" {
		existing.FullName = util.NormalizeName(req.FullName)
	}
	if req.PatientCode != "" {
		existing.PatientCode = req.PatientCode
	}
	if req.Email != "" {
		existing.Email = req.Email
	}
}

// updatePatientDetails updates non-identity profile fields
func updatePatientDetails(existing *model.Patient, req model.UpdatePatientRequest) {
	if req.Gender != "" {
		existing.Gender = req.Gender
	}
	if req.Age != 0 {
		existing.Age = req.Age
	}
	if req.Job != "" {
		existing.Job = req.Job
	}
	if req.Address != "" {
		existing.Address = req.Address
	}
	if req.HealthHistory != "" {
		existing.HealthHistory = req.HealthHistory
	}
	if req.SurgeryHistory != "" {
		existing.SurgeryHistory = req.SurgeryHistory
	}
}

// updatePatientPassword handles password hashing and update
func updatePatientPassword(existing *model.Patient, password string) {
	if password == "" {
		return
	}
	existing.Password = util.HashPassword(password)
}

func getInitials(fullName string) string {
	words := strings.Fields(fullName)
	initials := ""
	if len(words) > 0 && len(words[0]) > 0 {
		initials = strings.ToUpper(string(words[0][0]))
	}
	return initials
}

func getPatientByID(c *gin.Context, db *gorm.DB) (string, model.Patient, error) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing patient ID",
			Err: fmt.Errorf("patient ID is required"),
		})
		return "", model.Patient{}, fmt.Errorf("patient ID is required")
	}

	var patient model.Patient
	if err := db.First(&patient, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient not found",
			Err: err,
		})
		return "", model.Patient{}, err
	}

	return id, patient, nil
}

// DeletePatient godoc
// @Summary      Delete a patient
// @Description  Soft delete a patient by ID
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Patient ID"
// @Success      200 {object} util.APIResponse "Patient deleted"
// @Failure      400 {object} util.APIResponse "Patient not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient/{id} [delete]
func DeletePatient(c *gin.Context) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	_, patient, err := getPatientByID(c, db)
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

// GetPatientInfo godoc
// @Summary      Get patient information
// @Description  Get detailed information about a specific patient
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Patient ID"
// @Success      200 {object} util.APIResponse{data=model.Patient} "Patient retrieved"
// @Failure      400 {object} util.APIResponse "Patient not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient/{id} [get]
func GetPatientInfo(c *gin.Context) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	_, patient, err := getPatientByID(c, db)
	if err != nil {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient retrieved",
		Data: patient,
	})
}
