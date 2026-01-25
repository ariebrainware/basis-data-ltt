package endpoint

import (
	"bytes"
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
)

func doRequest(r http.Handler, method, path string, body []byte, headers map[string]string) (*httptest.ResponseRecorder, error) {
	req, err := http.NewRequest(method, path, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr, nil
}

func TestCreatePatient_InMemoryDB(t *testing.T) {
	// Setup test env
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret")
	t.Setenv("APITOKEN", "test-api-token")
	util.SetJWTSecret("test-secret")

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	// Migrate necessary models
	testModels := []interface{}{&model.Patient{}, &model.PatientCode{}, &model.Role{}, &model.User{}}
	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// Seed roles and a patient code for initial letter 'J' (for "John Doe")
	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.PatientCode{Alphabet: "J", Number: 1, Code: "J1"}).Error; err != nil {
		t.Fatalf("seed patient code: %v", err)
	}

	// Setup Gin router
	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))
	// public endpoint
	r.POST("/patient", CreatePatient)

	// Build request body
	patientBody := map[string]interface{}{
		"full_name":    "John Doe",
		"gender":       "Male",
		"age":          40,
		"job":          "Tester",
		"address":      "Test St",
		"email":        "johndoe@example.com",
		"phone_number": []string{"081200000"},
	}
	b, _ := json.Marshal(patientBody)

	rr, err := doRequest(r, "POST", "/patient", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify patient exists in DB
	var p model.Patient
	if err := db.Where("email = ?", "johndoe@example.com").First(&p).Error; err != nil {
		t.Fatalf("patient not found in DB: %v", err)
	}
	if p.PhoneNumber == "" {
		t.Fatalf("expected phone number stored, got empty")
	}
}

func TestCreatePatient_DuplicateDetection(t *testing.T) {
	// Setup test env
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret")
	t.Setenv("APITOKEN", "test-api-token")
	util.SetJWTSecret("test-secret")

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	// Migrate necessary models
	testModels := []interface{}{&model.Patient{}, &model.PatientCode{}, &model.Role{}, &model.User{}}
	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// Clean tables to ensure deterministic test
	if err := db.Unscoped().Where("1 = 1").Delete(&model.Patient{}).Error; err != nil {
		t.Fatalf("cleanup patients: %v", err)
	}
	if err := db.Unscoped().Where("1 = 1").Delete(&model.User{}).Error; err != nil {
		t.Fatalf("cleanup users: %v", err)
	}
	if err := db.Unscoped().Where("1 = 1").Delete(&model.PatientCode{}).Error; err != nil {
		t.Fatalf("cleanup patient codes: %v", err)
	}

	// Seed roles and a patient code for initial letter 'J' (for "John Doe")
	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.PatientCode{Alphabet: "J", Number: 1, Code: "J1"}).Error; err != nil {
		t.Fatalf("seed patient code: %v", err)
	}

	// Setup Gin router
	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))
	r.POST("/patient", CreatePatient)

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
	b, _ := json.Marshal(patientBody)
	rr, err := doRequest(r, "POST", "/patient", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for first creation, got %d: %s", rr.Code, rr.Body.String())
	}

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
	dbuf, _ := json.Marshal(dupBody)
	rr2, err := doRequest(r, "POST", "/patient", dbuf, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for duplicate, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if !strings.Contains(rr2.Body.String(), "Patient already exists") {
		t.Fatalf("expected duplicate message in response, got: %s", rr2.Body.String())
	}
}

func TestCreatePatient_DuplicateDetectionWithWhitespace(t *testing.T) {
	// Setup test env
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret")
	t.Setenv("APITOKEN", "test-api-token")
	util.SetJWTSecret("test-secret")

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	// Migrate necessary models
	testModels := []interface{}{&model.Patient{}, &model.PatientCode{}, &model.Role{}, &model.User{}}
	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// Clean tables to ensure deterministic test
	if err := db.Unscoped().Where("1 = 1").Delete(&model.Patient{}).Error; err != nil {
		t.Fatalf("cleanup patients: %v", err)
	}
	if err := db.Unscoped().Where("1 = 1").Delete(&model.User{}).Error; err != nil {
		t.Fatalf("cleanup users: %v", err)
	}
	if err := db.Unscoped().Where("1 = 1").Delete(&model.PatientCode{}).Error; err != nil {
		t.Fatalf("cleanup patient codes: %v", err)
	}

	// Seed roles and a patient code for initial letter 'J' (for "Jane Smith")
	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.PatientCode{Alphabet: "J", Number: 10, Code: "J10"}).Error; err != nil {
		t.Fatalf("seed patient code: %v", err)
	}

	// Setup Gin router
	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))
	r.POST("/patient", CreatePatient)

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
	b, _ := json.Marshal(patientBody)
	rr, err := doRequest(r, "POST", "/patient", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for first creation, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the patient was stored with normalized name (no leading/trailing spaces)
	var p model.Patient
	if err := db.Where("email = ?", "janesmith@example.com").First(&p).Error; err != nil {
		t.Fatalf("patient not found in DB: %v", err)
	}
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
	dbuf1, _ := json.Marshal(dupBody1)
	rr2, err := doRequest(r, "POST", "/patient", dbuf1, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for duplicate with whitespace, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if !strings.Contains(rr2.Body.String(), "Patient already exists") {
		t.Fatalf("expected duplicate message in response, got: %s", rr2.Body.String())
	}

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
	dbuf2, _ := json.Marshal(dupBody2)
	rr3, err := doRequest(r, "POST", "/patient", dbuf2, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("third request failed: %v", err)
	}
	if rr3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for duplicate with internal whitespace, got %d: %s", rr3.Code, rr3.Body.String())
	}
	if !strings.Contains(rr3.Body.String(), "Patient already exists") {
		t.Fatalf("expected duplicate message in response, got: %s", rr3.Body.String())
	}
}
