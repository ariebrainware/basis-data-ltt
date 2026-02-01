package endpoint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupTreatmentTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))
	return r, db
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

func TestListTreatments_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create test treatments
	createTestTreatment(db, t, "P001", 1)
	createTestTreatment(db, t, "P002", 1)

	r.GET("/treatment", ListTreatments)

	req := httptest.NewRequest(http.MethodGet, "/treatment", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestListTreatments_WithPagination(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create multiple treatments
	for i := 0; i < 5; i++ {
		createTestTreatment(db, t, fmt.Sprintf("P%03d", i), 1)
	}

	r.GET("/treatment", ListTreatments)

	req := httptest.NewRequest(http.MethodGet, "/treatment?limit=2&offset=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListTreatments_WithKeyword(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create patient with specific code
	patient := model.Patient{
		FullName:    "Search Patient",
		PatientCode: "SEARCH001",
		Email:       "search@test.com",
	}
	db.Create(&patient)

	createTestTreatment(db, t, "SEARCH001", 1)

	r.GET("/treatment", ListTreatments)

	req := httptest.NewRequest(http.MethodGet, "/treatment?keyword=SEARCH", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListTreatments_WithTherapistFilter(t *testing.T) {
	r, db := setupTreatmentTest(t)

	createTestTreatment(db, t, "P001", 1)
	createTestTreatment(db, t, "P002", 2)

	r.GET("/treatment", ListTreatments)

	req := httptest.NewRequest(http.MethodGet, "/treatment?therapist_id=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListTreatments_WithDateFilter(t *testing.T) {
	r, db := setupTreatmentTest(t)

	createTestTreatment(db, t, "P001", 1)

	r.GET("/treatment", ListTreatments)

	req := httptest.NewRequest(http.MethodGet, "/treatment?group_by_date=last_2_days", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListTreatments_WithSessionTherapist(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create user and session
	user := model.User{
		Name:     "Therapist User",
		Email:    "therapist@test.com",
		Password: "hash",
		RoleID:   2, // Therapist role
	}
	db.Create(&user)

	// Create therapist
	therapist := model.Therapist{
		FullName: "Session Therapist",
		NIK:      "SESS123",
		Email:    user.Email,
	}
	db.Create(&therapist)

	session := model.Session{
		UserID:       user.ID,
		SessionToken: "test-token-123",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	createTestTreatment(db, t, "P001", therapist.ID)

	r.GET("/treatment", ListTreatments)

	req := httptest.NewRequest(http.MethodGet, "/treatment", nil)
	req.Header.Set("session-token", "test-token-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateTreatment_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create patient
	patient := model.Patient{
		FullName:    "Test Patient",
		PatientCode: "CREATE001",
		Email:       "create@test.com",
	}
	db.Create(&patient)

	r.POST("/treatment", CreateTreatment)

	reqBody := map[string]interface{}{
		"patient_code": "CREATE001",
		"therapist_id": 1,
		"date":         time.Now().Format("2006-01-02"),
		"notes":        "Test treatment",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/treatment", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestCreateTreatment_InvalidJSON(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	r.POST("/treatment", CreateTreatment)

	req := httptest.NewRequest(http.MethodPost, "/treatment", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTreatment_DuplicateEntry(t *testing.T) {
	r, db := setupTreatmentTest(t)

	// Create patient
	patient := model.Patient{
		FullName:    "Test Patient",
		PatientCode: "DUP001",
		Email:       "dup@test.com",
	}
	db.Create(&patient)

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

	r.POST("/treatment", CreateTreatment)

	reqBody := map[string]interface{}{
		"patient_code": "DUP001",
		"therapist_id": 1,
		"date":         date,
		"notes":        "Duplicate treatment",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/treatment", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTreatment_PatientNotFound(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	r.POST("/treatment", CreateTreatment)

	reqBody := map[string]interface{}{
		"patient_code": "NOTEXIST",
		"therapist_id": 1,
		"date":         time.Now().Format("2006-01-02"),
		"notes":        "Test",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/treatment", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateTreatment_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	treatment := createTestTreatment(db, t, "UPD001", 1)

	r.PATCH("/treatment/:id", UpdateTreatment)

	reqBody := map[string]interface{}{
		"remarks": "Updated remarks",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/treatment/%d", treatment.ID), strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify update
	var updated model.Treatment
	db.First(&updated, treatment.ID)
	assert.Equal(t, "Updated remarks", updated.Remarks)
}

func TestUpdateTreatment_NotFound(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	r.PATCH("/treatment/:id", UpdateTreatment)

	reqBody := map[string]interface{}{
		"notes": "Updated",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/treatment/99999", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateTreatment_InvalidID(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	r.PATCH("/treatment/:id", UpdateTreatment)

	reqBody := map[string]interface{}{
		"notes": "Updated",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/treatment/invalid", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateTreatment_InvalidJSON(t *testing.T) {
	r, db := setupTreatmentTest(t)

	treatment := createTestTreatment(db, t, "INV001", 1)

	r.PATCH("/treatment/:id", UpdateTreatment)

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/treatment/%d", treatment.ID), strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteTreatment_Success(t *testing.T) {
	r, db := setupTreatmentTest(t)

	treatment := createTestTreatment(db, t, "DEL001", 1)

	r.DELETE("/treatment/:id", DeleteTreatment)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/treatment/%d", treatment.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify soft delete
	var deleted model.Treatment
	err := db.First(&deleted, treatment.ID).Error
	assert.Error(t, err)
}

func TestDeleteTreatment_NotFound(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	r.DELETE("/treatment/:id", DeleteTreatment)

	req := httptest.NewRequest(http.MethodDelete, "/treatment/99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteTreatment_InvalidID(t *testing.T) {
	r, db := setupTreatmentTest(t)
	_ = db

	r.DELETE("/treatment/:id", DeleteTreatment)

	req := httptest.NewRequest(http.MethodDelete, "/treatment/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseQueryInt_Valid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var result int
	r.GET("/test", func(c *gin.Context) {
		result = parseQueryIntTest(c, "value", 10)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test?value=25", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 25, result)
}

func TestParseQueryInt_Invalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var result int
	r.GET("/test", func(c *gin.Context) {
		result = parseQueryIntTest(c, "value", 10)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test?value=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 10, result) // Should return default
}

func TestParseQueryInt_Missing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var result int
	r.GET("/test", func(c *gin.Context) {
		result = parseQueryIntTest(c, "value", 10)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 10, result) // Should return default
}

func TestApplyCreatedAtFilterForTreatments(t *testing.T) {
	db := setupTestDB(t)

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
	db := setupTestDB(t)

	// Setup test data
	user := model.User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   2,
	}
	db.Create(&user)

	therapist := model.Therapist{
		FullName: "Test Therapist",
		NIK:      "TEST123",
		Email:    user.Email,
	}
	db.Create(&therapist)

	session := model.Session{
		UserID:       user.ID,
		SessionToken: "test-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	// Test
	id, err := getTherapistIDFromSession(db, "test-token")
	assert.NoError(t, err)
	assert.Equal(t, therapist.ID, id)
}

func TestGetTherapistIDFromSession_InvalidToken(t *testing.T) {
	db := setupTestDB(t)

	id, err := getTherapistIDFromSession(db, "invalid-token")
	assert.Error(t, err)
	assert.Equal(t, uint(0), id)
}

func TestGetTherapistIDFromSession_TherapistNotFound(t *testing.T) {
	db := setupTestDB(t)

	user := model.User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   2,
	}
	db.Create(&user)

	session := model.Session{
		UserID:       user.ID,
		SessionToken: "test-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	id, err := getTherapistIDFromSession(db, "test-token")
	assert.Error(t, err)
	assert.Equal(t, uint(0), id)
}

func TestHandleSessionError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/test", func(c *gin.Context) {
		handleSessionErrorTest(c, gorm.ErrRecordNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetDBOrAbort_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	var resultDB *gorm.DB
	var resultOK bool
	r.GET("/test", func(c *gin.Context) {
		resultDB, resultOK = getDBOrAbort(c)
		if resultOK {
			c.Status(http.StatusOK)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, resultOK)
	assert.NotNil(t, resultDB)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetDBOrAbort_NoDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var resultOK bool
	r.GET("/test", func(c *gin.Context) {
		_, resultOK = getDBOrAbort(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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
