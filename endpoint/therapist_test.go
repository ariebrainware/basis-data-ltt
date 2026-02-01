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

// helper to perform GET requests and parse JSON response
func performGetRequest(r *gin.Engine, url string) (*httptest.ResponseRecorder, map[string]interface{}, error) {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var response map[string]interface{}
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			return w, nil, err
		}
	}
	return w, response, nil
}

// helper to perform JSON requests (POST/PATCH/PUT) and parse JSON response
func performJSONRequest(r *gin.Engine, method, url string, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}, error) {
	var reader *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = strings.NewReader(string(b))
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, url, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var response map[string]interface{}
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			return w, nil, err
		}
	}
	return w, response, nil
}

// doGetWithHandler registers the GET handler at registerPath and performs a GET
// request against requestPath, returning the recorder and parsed JSON response.
func doGetWithHandler(r *gin.Engine, registerPath, requestPath string, handler gin.HandlerFunc) (*httptest.ResponseRecorder, map[string]interface{}, error) {
	r.GET(registerPath, handler)
	return performGetRequest(r, requestPath)
}

type jsonRequest struct {
	method       string
	registerPath string
	requestPath  string
	handler      gin.HandlerFunc
	body         interface{}
}

// doJSONWithHandler registers the handler for the given method at registerPath
// and performs a request against requestPath with the optional body.
func doJSONWithHandler(r *gin.Engine, req jsonRequest) (*httptest.ResponseRecorder, map[string]interface{}, error) {
	switch req.method {
	case http.MethodPost:
		r.POST(req.registerPath, req.handler)
	case http.MethodPatch:
		r.PATCH(req.registerPath, req.handler)
	case http.MethodPut:
		r.PUT(req.registerPath, req.handler)
	case http.MethodDelete:
		r.DELETE(req.registerPath, req.handler)
	default:
		r.Handle(req.method, req.registerPath, req.handler)
	}
	return performJSONRequest(r, req.method, req.requestPath, req.body)
}

func TestListTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create test therapists
	createTestTherapist(db, t, true)
	createTestTherapist(db, t, false)

	w, response, err := doGetWithHandler(r, "/therapist", "/therapist", ListTherapist)
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestListTherapist_WithPagination(t *testing.T) {
	r, db := setupTherapistTest(t)

	// Create multiple therapists
	for i := 0; i < 5; i++ {
		createTestTherapist(db, t, true)
	}

	w, response, err := doGetWithHandler(r, "/therapist", "/therapist?limit=2&offset=1", ListTherapist)
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

	w, response, err := doGetWithHandler(r, "/therapist", "/therapist?keyword=John", ListTherapist)
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestListTherapist_WithGroupByDate(t *testing.T) {
	r, db := setupTherapistTest(t)

	createTestTherapist(db, t, true)

	w, response, err := doGetWithHandler(r, "/therapist", "/therapist?group_by_date=last_2_days", ListTherapist)
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestGetTherapistInfo_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	w, response, err := doGetWithHandler(r, "/therapist/:id", fmt.Sprintf("/therapist/%d", therapist.ID), GetTherapistInfo)
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestGetTherapistInfo_NotFound(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	w, _, _ := doGetWithHandler(r, "/therapist/:id", "/therapist/99999", GetTherapistInfo)
	assertStatus(t, w, http.StatusNotFound)
}

func TestGetTherapistInfo_InvalidID(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	w, _, _ := doGetWithHandler(r, "/therapist/:id", "/therapist/invalid", GetTherapistInfo)
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

	w, response, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPost, registerPath: "/therapist", requestPath: "/therapist", handler: CreateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertSuccessResponse(t, w, response)
}

func TestCreateTherapist_MissingFields(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	reqBody := map[string]interface{}{
		"full_name": "Test",
	}
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPost, registerPath: "/therapist", requestPath: "/therapist", handler: CreateTherapist, body: reqBody})
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
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPost, registerPath: "/therapist", requestPath: "/therapist", handler: CreateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	reqBody := map[string]interface{}{
		"full_name": "Updated Name",
	}
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPatch, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: UpdateTherapist, body: reqBody})
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
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPatch, registerPath: "/therapist/:id", requestPath: "/therapist/99999", handler: UpdateTherapist, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusNotFound)
}

func TestUpdateTherapist_InvalidJSON(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	// Register handler and perform an invalid JSON request directly
	r.PATCH("/therapist/:id", UpdateTherapist)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/therapist/%d", therapist.ID), strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestTherapistApproval_Approve(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, false)

	reqBody := map[string]interface{}{
		"is_approved": true,
	}
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPut, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: TherapistApproval, body: reqBody})
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
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPut, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: TherapistApproval, body: reqBody})
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
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodPut, registerPath: "/therapist/:id", requestPath: "/therapist/99999", handler: TherapistApproval, body: reqBody})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteTherapist_Success(t *testing.T) {
	r, db := setupTherapistTest(t)

	therapist := createTestTherapist(db, t, true)

	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodDelete, registerPath: "/therapist/:id", requestPath: fmt.Sprintf("/therapist/%d", therapist.ID), handler: DeleteTherapist, body: nil})
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
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodDelete, registerPath: "/therapist/:id", requestPath: "/therapist/99999", handler: DeleteTherapist, body: nil})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteTherapist_InvalidID(t *testing.T) {
	r, db := setupTherapistTest(t)
	_ = db
	w, _, err := doJSONWithHandler(r, jsonRequest{method: http.MethodDelete, registerPath: "/therapist/:id", requestPath: "/therapist/invalid", handler: DeleteTherapist, body: nil})
	assert.NoError(t, err)
	assertStatus(t, w, http.StatusBadRequest)
}
