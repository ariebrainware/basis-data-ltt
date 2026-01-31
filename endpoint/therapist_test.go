package endpoint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// setupTestDBFull initializes database and migrates all required tables for therapist tests
func setupTestDBFull(t *testing.T) (*gorm.DB, error) {
	t.Helper()

	// Set test environment
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret-123")
	util.SetJWTSecret("test-secret-123")

	// Connect to test database
	db, err := config.ConnectMySQL()
	if err != nil {
		return nil, err
	}

	// Migrate all necessary tables for therapist tests
	testModels := []interface{}{
		&model.Patient{},
		&model.Disease{},
		&model.User{},
		&model.Session{},
		&model.Therapist{},
		&model.Role{},
		&model.Treatment{},
		&model.PatientCode{},
	}

	if err := db.AutoMigrate(testModels...); err != nil {
		return nil, err
	}

	// Clean up all tables
	for _, model := range testModels {
		db.Where("1 = 1").Delete(model)
	}

	// Register cleanup
	t.Cleanup(func() {
		// Drop tables after test completes
		for _, model := range testModels {
			_ = db.Migrator().DropTable(model)
		}
	})

	return db, nil
}

func setupTherapistTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Connect to test DB with full migration
	db, err := setupTestDBFull(t)
	if err != nil {
		t.Fatalf("setup test db failed: %v", err)
	}

	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))
	return r, db
}

func createTestTherapist(db *gorm.DB, t *testing.T, approved bool) model.Therapist {
	therapist := model.Therapist{
		FullName:   "Test Therapist",
		NIK:        fmt.Sprintf("NIK%d", time.Now().UnixNano()),
		Email:      fmt.Sprintf("therapist%d@test.com", time.Now().UnixNano()),
		IsApproved: approved,
	}
	err := db.Create(&therapist).Error
	assert.NoError(t, err)
	return therapist
}

func TestListTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create test therapists
	createTestTherapist(db, t, true)
	createTestTherapist(db, t, false)

	r.GET("/therapist", ListTherapist)

	req := httptest.NewRequest(http.MethodGet, "/therapist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestListTherapist_WithPagination(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create multiple therapists
	for i := 0; i < 5; i++ {
		createTestTherapist(db, t, true)
	}

	r.GET("/therapist", ListTherapist)

	req := httptest.NewRequest(http.MethodGet, "/therapist?limit=2&offset=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestListTherapist_WithKeyword(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create therapist with specific name
	therapist := model.Therapist{
		FullName:   "John Doe Therapist",
		NIK:        "SEARCH123",
		Email:      "search@test.com",
		IsApproved: true,
	}
	db.Create(&therapist)

	r.GET("/therapist", ListTherapist)

	req := httptest.NewRequest(http.MethodGet, "/therapist?keyword=John", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestListTherapist_WithGroupByDate(t *testing.T) {
	r, db := setupTherapistTest(t)

	createTestTherapist(db, t, true)

	r.GET("/therapist", ListTherapist)

	req := httptest.NewRequest(http.MethodGet, "/therapist?group_by_date=last_2_days", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestGetTherapistInfo_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	r.GET("/therapist/:id", GetTherapistInfo)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/therapist/%d", therapist.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestGetTherapistInfo_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.GET("/therapist/:id", GetTherapistInfo)

	req := httptest.NewRequest(http.MethodGet, "/therapist/99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetTherapistInfo_InvalidID(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.GET("/therapist/:id", GetTherapistInfo)

	req := httptest.NewRequest(http.MethodGet, "/therapist/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.POST("/therapist", CreateTherapist)

	reqBody := map[string]interface{}{
		"full_name": "New Therapist",
		"nik":       fmt.Sprintf("NIK%d", time.Now().UnixNano()),
		"email":     fmt.Sprintf("new%d@test.com", time.Now().UnixNano()),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/therapist", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestCreateTherapist_MissingFields(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.POST("/therapist", CreateTherapist)

	reqBody := map[string]interface{}{
		"full_name": "Test",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/therapist", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTherapist_DuplicateNIK(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create existing therapist
	existing := model.Therapist{
		FullName: "Existing",
		NIK:      "DUPLICATE123",
		Email:    "existing@test.com",
	}
	db.Create(&existing)

	r.POST("/therapist", CreateTherapist)

	reqBody := map[string]interface{}{
		"full_name": "New",
		"nik":       "DUPLICATE123",
		"email":     "new@test.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/therapist", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	r.PATCH("/therapist/:id", UpdateTherapist)

	reqBody := map[string]interface{}{
		"full_name": "Updated Name",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/therapist/%d", therapist.ID), strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify update
	var updated model.Therapist
	db.First(&updated, therapist.ID)
	assert.Equal(t, "Updated Name", updated.FullName)
}

func TestUpdateTherapist_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.PATCH("/therapist/:id", UpdateTherapist)

	reqBody := map[string]interface{}{
		"full_name": "Updated",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/therapist/99999", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateTherapist_InvalidJSON(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	r.PATCH("/therapist/:id", UpdateTherapist)

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/therapist/%d", therapist.ID), strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTherapistApproval_Approve(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, false)

	r.PUT("/therapist/:id", TherapistApproval)

	reqBody := map[string]interface{}{
		"is_approved": true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/therapist/%d", therapist.ID), strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify approval
	var updated model.Therapist
	db.First(&updated, therapist.ID)
	assert.True(t, updated.IsApproved)
}

func TestTherapistApproval_Reject(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	r.PUT("/therapist/:id", TherapistApproval)

	reqBody := map[string]interface{}{
		"is_approved": false,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/therapist/%d", therapist.ID), strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify rejection
	var updated model.Therapist
	db.First(&updated, therapist.ID)
	assert.False(t, updated.IsApproved)
}

func TestTherapistApproval_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.PUT("/therapist/:id", TherapistApproval)

	reqBody := map[string]interface{}{
		"is_approved": true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/therapist/99999", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	r.DELETE("/therapist/:id", DeleteTherapist)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/therapist/%d", therapist.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify soft delete
	var deleted model.Therapist
	err := db.First(&deleted, therapist.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestDeleteTherapist_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.DELETE("/therapist/:id", DeleteTherapist)

	req := httptest.NewRequest(http.MethodDelete, "/therapist/99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteTherapist_InvalidID(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db

	r.DELETE("/therapist/:id", DeleteTherapist)

	req := httptest.NewRequest(http.MethodDelete, "/therapist/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
