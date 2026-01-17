package endpoint

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
