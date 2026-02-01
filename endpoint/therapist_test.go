package endpoint

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	assert.Equal(t, expected, w.Code)
}

func assertSuccessResponse(t *testing.T, w *httptest.ResponseRecorder, response map[string]interface{}) {
	t.Helper()
	assert.Equal(t, http.StatusOK, w.Code)
	if response == nil {
		return
	}
	if success, ok := response["success"].(bool); ok {
		assert.True(t, success)
	}
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

// request helpers moved to test_request_helpers_test.go

func TestListTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create test therapists
	createTestTherapist(db, t, true)
	createTestTherapist(db, t, false)

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/therapist", requestPath: "/therapist", handler: ListTherapist})
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestListTherapist_WithPagination(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create multiple therapists
	for i := 0; i < 5; i++ {
		createTestTherapist(db, t, true)
	}

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/therapist", requestPath: "/therapist?limit=2&offset=1", handler: ListTherapist})
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
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

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/therapist", requestPath: "/therapist?keyword=John", handler: ListTherapist})
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestListTherapist_WithGroupByDate(t *testing.T) {
	r, db := setupTherapistTest(t)

	createTestTherapist(db, t, true)

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/therapist", requestPath: "/therapist?group_by_date=last_2_days", handler: ListTherapist})
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestGetTherapistInfo_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: GetTherapistInfo})
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestGetTherapistInfo_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	w, _, _ := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/therapist/:id", requestPath: "/therapist/99999", handler: GetTherapistInfo})
	assertStatus(t, w, http.StatusNotFound)
}

func TestGetTherapistInfo_InvalidID(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	w, _, _ := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/therapist/:id", requestPath: "/therapist/invalid", handler: GetTherapistInfo})
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	reqBody := map[string]interface{}{
		"full_name": "New Therapist",
		"nik":       fmt.Sprintf("NIK%d", time.Now().UnixNano()),
		"email":     fmt.Sprintf("new%d@test.com", time.Now().UnixNano()),
	}

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodPost, registerPath: "/therapist", requestPath: "/therapist", handler: CreateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestCreateTherapist_MissingFields(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	reqBody := map[string]interface{}{
		"full_name": "Test",
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPost, registerPath: "/therapist", requestPath: "/therapist", handler: CreateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusBadRequest)
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

	reqBody := map[string]interface{}{
		"full_name": "New",
		"nik":       "DUPLICATE123",
		"email":     "new@test.com",
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPost, registerPath: "/therapist", requestPath: "/therapist", handler: CreateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	reqBody := map[string]interface{}{
		"full_name": "Updated Name",
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPatch, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: UpdateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusOK)

	// Verify update
	var updated model.Therapist
	db.First(&updated, therapist.ID)
	assert.Equal(t, "Updated Name", updated.FullName)
}

func TestUpdateTherapist_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	reqBody := map[string]interface{}{
		"full_name": "Updated",
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPatch, registerPath: "/therapist/:id", requestPath: "/therapist/99999", handler: UpdateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusNotFound)
}

func TestUpdateTherapist_InvalidJSON(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPatch, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: UpdateTherapist, body: "invalid json"})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestTherapistApproval_Approve(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, false)

	reqBody := map[string]interface{}{
		"is_approved": true,
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPut, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: TherapistApproval, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusOK)

	// Verify approval
	var updated model.Therapist
	db.First(&updated, therapist.ID)
	assert.True(t, updated.IsApproved)
}

func TestTherapistApproval_Reject(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	reqBody := map[string]interface{}{
		"is_approved": false,
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPut, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: TherapistApproval, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusOK)

	// Verify rejection
	var updated model.Therapist
	db.First(&updated, therapist.ID)
	assert.False(t, updated.IsApproved)
}

func TestTherapistApproval_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	reqBody := map[string]interface{}{
		"is_approved": true,
	}
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodPut, registerPath: "/therapist/:id", requestPath: "/therapist/99999", handler: TherapistApproval, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodDelete, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: DeleteTherapist, body: nil})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusOK)

	// Verify soft delete
	var deleted model.Therapist
	err = db.First(&deleted, therapist.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestDeleteTherapist_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodDelete, registerPath: "/therapist/:id", requestPath: "/therapist/99999", handler: DeleteTherapist, body: nil})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteTherapist_InvalidID(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodDelete, registerPath: "/therapist/:id", requestPath: "/therapist/invalid", handler: DeleteTherapist, body: nil})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusBadRequest)
}
