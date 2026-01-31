package endpoint

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type testSetupParams struct {
	secret   string
	apiToken string
}

func setupTestEnv(t *testing.T, params testSetupParams) (*config.Config, *gorm.DB) {
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", params.secret)
	t.Setenv("APITOKEN", params.apiToken)
	util.SetJWTSecret(params.secret)

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	testModels := []interface{}{&model.Patient{}, &model.PatientCode{}, &model.Role{}, &model.User{}}
	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return cfg, db
}

func cleanupTestData(t *testing.T, db *gorm.DB) {
	tables := []interface{}{&model.Patient{}, &model.User{}, &model.PatientCode{}}
	for _, table := range tables {
		if err := db.Unscoped().Where("1 = 1").Delete(table).Error; err != nil {
			t.Fatalf("cleanup table: %v", err)
		}
	}
}

func setupTestRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))
	r.POST("/patient", CreatePatient)
	return r
}

func sendPatientRequest(r *gin.Engine, patientData map[string]interface{}) (*httptest.ResponseRecorder, error) {
	b, _ := json.Marshal(patientData)
	return doRequest(r, requestParams{
		method:  "POST",
		path:    "/patient",
		body:    b,
		headers: map[string]string{"Authorization": "Bearer test-api-token"},
	})
}

func assertResponseStatus(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int, msg string) {
	if rr.Code != expectedStatus {
		t.Fatalf(msg, rr.Code, expectedStatus, rr.Body.String())
	}
}

func assertPatientExists(t *testing.T, db *gorm.DB, email string) model.Patient {
	var p model.Patient
	if err := db.Where("email = ?", email).First(&p).Error; err != nil {
		t.Fatalf("patient not found in DB: %v", err)
	}
	return p
}

func assertDuplicateResponse(t *testing.T, rr *httptest.ResponseRecorder, bodyContains string) {
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), bodyContains) {
		t.Fatalf("expected response to contain '%s', got: %s", bodyContains, rr.Body.String())
	}
}

func TestCreatePatient_InMemoryDB(t *testing.T) {
	cfg, db := setupTestEnv(t, testSetupParams{
		secret:   "test-secret",
		apiToken: "test-api-token",
	})
	cleanupTestData(t, db)

	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.PatientCode{Alphabet: "J", Number: 1, Code: "J1"}).Error; err != nil {
		t.Fatalf("seed patient code: %v", err)
	}

	r := setupTestRouter(cfg, db)

	patientBody := map[string]interface{}{
		"full_name":    "John Doe",
		"gender":       "Male",
		"age":          40,
		"job":          "Tester",
		"address":      "Test St",
		"email":        "johndoe@example.com",
		"phone_number": []string{"081200000"},
	}
	rr, err := sendPatientRequest(r, patientBody)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	assertResponseStatus(t, rr, http.StatusOK, "expected 200 OK, got %d (expected %d): %s")

	p := assertPatientExists(t, db, "johndoe@example.com")
	if p.PhoneNumber == "" {
		t.Fatalf("expected phone number stored, got empty")
	}
}

func TestCreatePatient_DuplicateDetection(t *testing.T) {
	cfg, db := setupTestEnv(t, testSetupParams{
		secret:   "test-secret",
		apiToken: "test-api-token",
	})
	cleanupTestData(t, db)

	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.PatientCode{Alphabet: "J", Number: 1, Code: "J1"}).Error; err != nil {
		t.Fatalf("seed patient code: %v", err)
	}

	r := setupTestRouter(cfg, db)

	// First creation should succeed
	patientBody := map[string]interface{}{
		"full_name":    "John Doe",
		"gender":       "Male",
		"age":          40,
		"job":          "Tester",
		"address":      "Test St",
		"email":        "johndoe@example.com",
		"phone_number": []string{"081200000"},
	}
	rr, err := sendPatientRequest(r, patientBody)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	assertResponseStatus(t, rr, http.StatusOK, "expected 200 OK for first creation, got %d (expected %d): %s")

	// Second creation with same full_name + phone (with surrounding spaces) should be rejected
	dupBody := map[string]interface{}{
		"full_name":    "John Doe",
		"gender":       "Male",
		"age":          41,
		"job":          "Tester",
		"address":      "Other St",
		"email":        "johndoe2@example.com",
		"phone_number": []string{" 081200000 "},
	}
	rr2, err := sendPatientRequest(r, dupBody)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	assertDuplicateResponse(t, rr2, "Patient already exists")
}

func TestCreatePatient_DuplicateDetectionWithWhitespace(t *testing.T) {
	cfg, db := setupTestEnv(t, testSetupParams{
		secret:   "test-secret",
		apiToken: "test-api-token",
	})
	cleanupTestData(t, db)

	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.PatientCode{Alphabet: "J", Number: 10, Code: "J10"}).Error; err != nil {
		t.Fatalf("seed patient code: %v", err)
	}

	r := setupTestRouter(cfg, db)

	// First creation with "Jane Smith" should succeed
	patientBody := map[string]interface{}{
		"full_name":    "Jane Smith",
		"gender":       "Female",
		"age":          35,
		"job":          "Engineer",
		"address":      "Test Ave",
		"email":        "janesmith@example.com",
		"phone_number": []string{"081300000"},
	}
	rr, err := sendPatientRequest(r, patientBody)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	assertResponseStatus(t, rr, http.StatusOK, "expected 200 OK for first creation, got %d (expected %d): %s")

	// Verify the patient was stored with normalized name (no leading/trailing spaces)
	p := assertPatientExists(t, db, "janesmith@example.com")
	if p.FullName != "Jane Smith" {
		t.Errorf("expected normalized full_name 'Jane Smith', got '%s'", p.FullName)
	}

	// Second creation with leading/trailing whitespace in full_name should be rejected
	dupBody1 := map[string]interface{}{
		"full_name":    " Jane Smith ",
		"gender":       "Female",
		"age":          36,
		"job":          "Engineer",
		"address":      "Other Ave",
		"email":        "janesmith2@example.com",
		"phone_number": []string{"081300000"},
	}
	rr2, err := sendPatientRequest(r, dupBody1)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	assertDuplicateResponse(t, rr2, "Patient already exists")

	// Third creation with extra internal whitespace should also be rejected
	dupBody2 := map[string]interface{}{
		"full_name":    "Jane  Smith",
		"gender":       "Female",
		"age":          37,
		"job":          "Engineer",
		"address":      "Another Ave",
		"email":        "janesmith3@example.com",
		"phone_number": []string{"081300000"},
	}
	rr3, err := sendPatientRequest(r, dupBody2)
	if err != nil {
		t.Fatalf("third request failed: %v", err)
	}
	assertDuplicateResponse(t, rr3, "Patient already exists")
}
