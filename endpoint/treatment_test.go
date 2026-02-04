package endpoint

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupTreatmentTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db := setupTreatmentDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))
	return r, db
}

// newTestRouter returns a new Gin engine configured for tests.
// Use this for tests that don't need a DB injected.
func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func setupTreatmentDB(t *testing.T) *gorm.DB {
	t.Helper()

	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret")
	util.SetJWTSecret("test-secret")

	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	models := []interface{}{
		&model.Patient{},
		&model.Therapist{},
		&model.Treatment{},
		&model.User{},
		&model.Session{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	for _, m := range models {
		db.Where("1 = 1").Delete(m)
	}

	t.Cleanup(func() {
		_ = db.Migrator().DropTable(models...)
	})

	return db
}

func createTestTreatment(db *gorm.DB, t *testing.T, patientCode string, therapistID uint) model.Treatment {
	// Create patient if not exists
	var patient model.Patient
	if err := db.Where("patient_code = ?", patientCode).First(&patient).Error; err != nil {
		patient = model.Patient{
			FullName:    "Test Patient",
			PatientCode: patientCode,
			Email:       fmt.Sprintf("patient%d@test.com", time.Now().UnixNano()),
		}
		db.Create(&patient)
	}

	// Create therapist if not exists
	var therapist model.Therapist
	if therapistID > 0 {
		if err := db.First(&therapist, therapistID).Error; err != nil {
			therapist = model.Therapist{
				FullName: "Test Therapist",
				NIK:      fmt.Sprintf("NIK%d", time.Now().UnixNano()),
				Email:    fmt.Sprintf("therapist%d@test.com", time.Now().UnixNano()),
			}
			therapist.ID = therapistID
			db.Create(&therapist)
		}
	}

	treatment := model.Treatment{
		PatientCode:   patientCode,
		TherapistID:   therapistID,
		TreatmentDate: time.Now().Format("2006-01-02"),
		Issues:        "Test issues",
		Treatment:     "Test treatment",
		Remarks:       "Test remarks",
		NextVisit:     time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
	}
	err := db.Create(&treatment).Error
	assert.NoError(t, err)
	return treatment
}

// CreateUserSessionOpts groups creation options for createUserWithSession to
// avoid excess function arguments and improve readability in tests.
type CreateUserSessionOpts struct {
	RoleID          uint32
	Email           string
	Token           string
	CreateTherapist bool
}

// createUserWithSession creates a user according to opts, optionally a therapist
// record that references the user's email, and a session with the provided token.
// Returns the created user, therapist (may be zero value), and session (may be zero value).
func createUserWithSession(db *gorm.DB, t *testing.T, opts CreateUserSessionOpts) (model.User, model.Therapist, model.Session) {
	t.Helper()
	user := model.User{
		Name:     "Therapist User",
		Email:    opts.Email,
		Password: "hash",
		RoleID:   opts.RoleID,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	var therapist model.Therapist
	if opts.CreateTherapist {
		therapist = model.Therapist{
			FullName: "Session Therapist",
			NIK:      fmt.Sprintf("NIK%d", time.Now().UnixNano()),
			Email:    user.Email,
		}
		if err := db.Create(&therapist).Error; err != nil {
			t.Fatalf("failed to create therapist: %v", err)
		}
	}

	var session model.Session
	if opts.Token != "" {
		session = model.Session{
			UserID:       user.ID,
			SessionToken: opts.Token,
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		if err := db.Create(&session).Error; err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
	}

	return user, therapist, session
}

// test helpers to reduce duplication
func createPatientIfNotExists(db *gorm.DB, t *testing.T, patientCode, email string) model.Patient {
	t.Helper()
	var patient model.Patient
	if err := db.Where("patient_code = ?", patientCode).First(&patient).Error; err != nil {
		patient = model.Patient{
			FullName:    "Test Patient",
			PatientCode: patientCode,
			Email:       email,
		}
		if err := db.Create(&patient).Error; err != nil {
			t.Fatalf("failed to create patient: %v", err)
		}
	}
	return patient
}

// TreatmentRequestOpts groups common treatment request parameters to avoid duplication.
// Use zero values for defaults.
type TreatmentRequestOpts struct {
	PatientCode   string
	TherapistID   uint
	TreatmentDate string // defaults to today if empty
	Issues        string
	Treatment     []string
	Remarks       string
	NextVisit     string // defaults to 7 days from now if empty
}

// buildTreatmentRequest constructs a standard treatment request body from opts.
// Applies sensible defaults for empty fields.
func buildTreatmentRequest(opts TreatmentRequestOpts) map[string]interface{} {
	if opts.TreatmentDate == "" {
		opts.TreatmentDate = time.Now().Format("2006-01-02")
	}
	if opts.NextVisit == "" {
		opts.NextVisit = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}
	if len(opts.Treatment) == 0 {
		opts.Treatment = []string{"Test treatment"}
	}
	if opts.Issues == "" {
		opts.Issues = "Test issues"
	}
	if opts.Remarks == "" {
		opts.Remarks = "Test remarks"
	}

	return map[string]interface{}{
		"patient_code":   opts.PatientCode,
		"therapist_id":   opts.TherapistID,
		"treatment_date": opts.TreatmentDate,
		"issues":         opts.Issues,
		"treatment":      opts.Treatment,
		"remarks":        opts.Remarks,
		"next_visit":     opts.NextVisit,
	}
}

// assertTreatmentSuccessResponse checks that response is successful and status is 200.
func assertTreatmentSuccessResponse(t *testing.T, w *httptest.ResponseRecorder, response map[string]interface{}) {
	t.Helper()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))
}

// assertTreatmentErrorResponse checks that response is an error with the expected status code.
func assertTreatmentErrorResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()
	assert.Equal(t, expectedStatus, w.Code)
}

// assertStatusWithError checks response status and that no error occurred during request.
// Commonly used in update/delete error path tests.
func assertStatusWithError(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, err error) {
	t.Helper()
	assert.Equal(t, expectedStatus, w.Code)
	assert.NoError(t, err)
}

// request helpers moved to test_request_helpers_test.go for reuse

func TestListTreatments_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create test treatments
	createTestTreatment(db, t, "P001", 1)
	createTestTreatment(db, t, "P002", 1)

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/treatment", requestPath: "/treatment", handler: ListTreatments})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestListTreatments_WithPagination(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create multiple treatments
	for i := 0; i < 5; i++ {
		createTestTreatment(db, t, fmt.Sprintf("P%03d", i), 1)
	}

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/treatment", requestPath: "/treatment?limit=2&offset=1", handler: ListTreatments})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)
}

func TestListTreatments_WithKeyword(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create patient with specific code
	_ = createPatientIfNotExists(db, t, "SEARCH001", "search@test.com")

	createTestTreatment(db, t, "SEARCH001", 1)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/treatment", requestPath: "/treatment?keyword=SEARCH", handler: ListTreatments})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)
}

func TestListTreatments_WithTherapistFilter(t *testing.T) {
	r, db := setupTreatmentTest(t)

	createTestTreatment(db, t, "P001", 1)
	createTestTreatment(db, t, "P002", 2)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/treatment", requestPath: "/treatment?therapist_id=1", handler: ListTreatments})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)
}

func TestListTreatments_WithDateFilter(t *testing.T) {
	r, db := setupTreatmentTest(t)

	createTestTreatment(db, t, "P001", 1)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/treatment", requestPath: "/treatment?group_by_date=last_2_days", handler: ListTreatments})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)
}

func TestListTreatments_WithSessionTherapist(t *testing.T) {
	r, db := setupTreatmentTest(t)
	// Create user, therapist and session
	_, therapist, session := createUserWithSession(db, t, CreateUserSessionOpts{RoleID: 2, Email: "therapist@test.com", Token: "test-token-123", CreateTherapist: true})

	createTestTreatment(db, t, "P001", therapist.ID)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/treatment", requestPath: "/treatment", handler: ListTreatments, headers: map[string]string{"session-token": session.SessionToken}})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)
}

func TestCreateTreatment_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create patient
	_ = createPatientIfNotExists(db, t, "CREATE001", "create@test.com")

	reqBody := buildTreatmentRequest(TreatmentRequestOpts{
		PatientCode: "CREATE001",
		TherapistID: 1,
	})
	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodPost, registerPath: "/treatment", requestPath: "/treatment", handler: CreateTreatment, body: reqBody})

	assert.NoError(t, err)
	assertTreatmentSuccessResponse(t, w, response)
}

func TestCreateTreatment_InvalidJSON(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPost, registerPath: "/treatment", requestPath: "/treatment", handler: CreateTreatment, body: "invalid json"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NoError(t, err)
}

func TestCreateTreatment_DuplicateEntry(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create patient
	_ = createPatientIfNotExists(db, t, "DUP001", "dup@test.com")

	date := time.Now().Format("2006-01-02")

	// Create first treatment
	treatment := model.Treatment{
		PatientCode:   "DUP001",
		TherapistID:   1,
		TreatmentDate: date,
		Issues:        "Issue 1",
		Treatment:     "First treatment",
		Remarks:       "First session",
		NextVisit:     time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
	}
	db.Create(&treatment)

	reqBody := buildTreatmentRequest(TreatmentRequestOpts{
		PatientCode:   "DUP001",
		TherapistID:   1,
		TreatmentDate: date,
		Issues:        "Issue 1",
		Treatment:     []string{"Duplicate treatment"},
		Remarks:       "Duplicate remarks",
	})
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPost, registerPath: "/treatment", requestPath: "/treatment", handler: CreateTreatment, body: reqBody})

	assertTreatmentErrorResponse(t, w, http.StatusBadRequest)
	assert.NoError(t, err)
}

func TestCreateTreatment_PatientNotFound(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	reqBody := buildTreatmentRequest(TreatmentRequestOpts{
		PatientCode: "NOTEXIST",
		TherapistID: 1,
	})
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPost, registerPath: "/treatment", requestPath: "/treatment", handler: CreateTreatment, body: reqBody})

	assertTreatmentErrorResponse(t, w, http.StatusBadRequest)
	assert.NoError(t, err)
}

func TestUpdateTreatment_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	treatment := createTestTreatment(db, t, "UPD001", 1)

	reqBody := map[string]interface{}{
		"remarks": "Updated remarks",
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPatch, registerPath: "/treatment/:id", requestPath: fmt.Sprintf("/treatment/%d", treatment.ID), handler: UpdateTreatment, body: reqBody})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)

	// Verify update
	var updated model.Treatment
	db.First(&updated, treatment.ID)
	assert.Equal(t, "Updated remarks", updated.Remarks)
}

func TestUpdateTreatment_NotFound(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	reqBody := map[string]interface{}{
		"notes": "Updated",
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPatch, registerPath: "/treatment/:id", requestPath: "/treatment/99999", handler: UpdateTreatment, body: reqBody})

	assertStatusWithError(t, w, http.StatusBadRequest, err)
}

func TestUpdateTreatment_InvalidID(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	reqBody := map[string]interface{}{
		"notes": "Updated",
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPatch, registerPath: "/treatment/:id", requestPath: "/treatment/invalid", handler: UpdateTreatment, body: reqBody})

	assertStatusWithError(t, w, http.StatusBadRequest, err)
}

func TestUpdateTreatment_InvalidJSON(t *testing.T) {
	r, db := setupTreatmentTest(t)

	treatment := createTestTreatment(db, t, "INV001", 1)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPatch, registerPath: "/treatment/:id", requestPath: fmt.Sprintf("/treatment/%d", treatment.ID), handler: UpdateTreatment, body: "invalid json"})

	assertStatusWithError(t, w, http.StatusBadRequest, err)
}

func TestDeleteTreatment_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	treatment := createTestTreatment(db, t, "DEL001", 1)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodDelete, registerPath: "/treatment/:id", requestPath: fmt.Sprintf("/treatment/%d", treatment.ID), handler: DeleteTreatment})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, err)

	// Verify soft delete
	var deleted model.Treatment
	err = db.First(&deleted, treatment.ID).Error
	assert.Error(t, err)
}

func TestDeleteTreatment_NotFound(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodDelete, registerPath: "/treatment/:id", requestPath: "/treatment/99999", handler: DeleteTreatment})

	assertStatusWithError(t, w, http.StatusBadRequest, err)
}

func TestDeleteTreatment_InvalidID(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodDelete, registerPath: "/treatment/:id", requestPath: "/treatment/invalid", handler: DeleteTreatment})

	assertStatusWithError(t, w, http.StatusBadRequest, err)
}

func TestParseQueryInt_Valid(t *testing.T) {
	r := newTestRouter()
	var result int
	handler := func(c *gin.Context) {
		result = parseQueryIntTest(c, "value", 10)
		c.Status(http.StatusOK)
	}

	_, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/test", requestPath: "/test?value=25", handler: handler})
	assert.NoError(t, err)
	assert.Equal(t, 25, result)
}

func TestParseQueryInt_Invalid(t *testing.T) {
	r := newTestRouter()
	var result int
	handler := func(c *gin.Context) {
		result = parseQueryIntTest(c, "value", 10)
		c.Status(http.StatusOK)
	}

	_, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/test", requestPath: "/test?value=invalid", handler: handler})
	assert.NoError(t, err)
	assert.Equal(t, 10, result) // Should return default
}

func TestParseQueryInt_Missing(t *testing.T) {
	r := newTestRouter()
	var result int
	handler := func(c *gin.Context) {
		result = parseQueryIntTest(c, "value", 10)
		c.Status(http.StatusOK)
	}

	_, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/test", requestPath: "/test", handler: handler})
	assert.NoError(t, err)
	assert.Equal(t, 10, result) // Should return default
}

func TestApplyCreatedAtFilterForTreatments(t *testing.T) {
	db := setupTreatmentDB(t)

	tests := []struct {
		name     string
		filter   string
		expected bool
	}{
		{"last_2_days", "last_2_days", true},
		{"last_3_months", "last_3_months", true},
		{"last_6_months", "last_6_months", true},
		{"invalid", "invalid", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := db.Model(&model.Treatment{})
			result := applyCreatedAtFilterForTreatments(query, tt.filter)
			assert.NotNil(t, result)
		})
	}
}

func TestGetTherapistIDFromSession_Success(t *testing.T) {
	db := setupTreatmentDB(t)

	// Setup test data
	_, therapist, session := createUserWithSession(db, t, CreateUserSessionOpts{RoleID: 2, Email: "test@test.com", Token: "test-token", CreateTherapist: true})

	// Test
	id, err := getTherapistIDFromSession(db, session.SessionToken)
	assert.NoError(t, err)
	assert.Equal(t, therapist.ID, id)
}

func TestGetTherapistIDFromSession_InvalidToken(t *testing.T) {
	db := setupTreatmentDB(t)

	id, err := getTherapistIDFromSession(db, "invalid-token")
	assert.Error(t, err)
	assert.Equal(t, uint(0), id)
}

func TestGetTherapistIDFromSession_TherapistNotFound(t *testing.T) {
	db := setupTreatmentDB(t)

	// Create user and session but no therapist record
	_, _, session := createUserWithSession(db, t, CreateUserSessionOpts{RoleID: 2, Email: "test@test.com", Token: "test-token", CreateTherapist: false})

	id, err := getTherapistIDFromSession(db, session.SessionToken)
	assert.Error(t, err)
	assert.Equal(t, uint(0), id)
}

func TestHandleSessionError(t *testing.T) {
	r := newTestRouter()
	handler := func(c *gin.Context) {
		handleSessionErrorTest(c, gorm.ErrRecordNotFound)
	}

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/test", requestPath: "/test", handler: handler})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetDBOrAbort_Success(t *testing.T) {
	db := setupTreatmentDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	var resultDB *gorm.DB
	var resultOK bool
	handler := func(c *gin.Context) {
		resultDB, resultOK = getDBOrAbort(c)
		if resultOK {
			c.Status(http.StatusOK)
		}
	}

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/test", requestPath: "/test", handler: handler})
	assert.NoError(t, err)
	assert.True(t, resultOK)
	assert.NotNil(t, resultDB)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetDBOrAbort_NoDatabase(t *testing.T) {
	r := newTestRouter()

	var resultOK bool
	handler := func(c *gin.Context) {
		_, resultOK = getDBOrAbort(c)
	}

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/test", requestPath: "/test", handler: handler})
	_ = w
	assert.NoError(t, err)
	assert.False(t, resultOK)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Test helper functions
func parseQueryIntTest(c *gin.Context, key string, defaultValue int) int {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return result
}

func handleSessionErrorTest(c *gin.Context, err error) {
	if err == gorm.ErrRecordNotFound {
		util.CallUserNotAuthorized(c, util.APIErrorParams{
			Msg: "Invalid or expired session",
			Err: err,
		})
		return
	}
	util.CallServerError(c, util.APIErrorParams{
		Msg: "Session lookup failed",
		Err: err,
	})
}

// Initialize JWT secret for tests
func init() {
	util.SetJWTSecret("test-secret-key-for-treatment-tests")
}
