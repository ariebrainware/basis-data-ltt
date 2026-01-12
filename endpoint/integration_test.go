package endpoint_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/endpoint"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

type apiResp struct {
	Success bool            `json:"success"`
	Error   string          `json:"error"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

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

func TestIntegrationFlow(t *testing.T) {
	// Set environment for test run using t.Setenv for test isolation
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret-123")
	t.Setenv("APITOKEN", "test-api-token")
	t.Setenv("GINMODE", "release")

	// Ensure util uses the test secret
	util.SetJWTSecret("test-secret-123")

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

	// Clean up database state at the end of the test
	t.Cleanup(func() {
		// Drop all tables to ensure clean state for next test run
		db.Migrator().DropTable(&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.PatientCode{})
	})

	// Auto migrate models used in tests
	if err := db.AutoMigrate(&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.PatientCode{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seeding roles failed: %v", err)
	}

	// Ensure there's an initial patient code entry for initials 'P' used by "Patient One"
	if err := db.Create(&model.PatientCode{Alphabet: "P", Number: 1, Code: "P1"}).Error; err != nil {
		t.Fatalf("failed to seed patient code: %v", err)
	}

	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))

	// Public endpoints
	r.POST("/signup", endpoint.Signup)
	r.POST("/login", endpoint.Login)
	r.GET("/token/validate", endpoint.ValidateToken)
	r.POST("/patient", endpoint.CreatePatient)

	// Protected logout endpoint
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		auth.DELETE("/logout", endpoint.Logout)
	}

	// 1) Signup
	signupBody := map[string]string{"name": "Test User", "email": "test@example.com", "password": "pass123"}
	b, _ := json.Marshal(signupBody)
	rr, err := doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup returned non-200: %d %s", rr.Code, rr.Body.String())
	}
	var resp apiResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode signup response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("signup returned success=false: %s", string(resp.Data))
	}

	var signupToken string
	if err := json.Unmarshal(resp.Data, &signupToken); err != nil {
		t.Fatalf("failed to parse signup token: %v", err)
	}

	// 2) Login
	loginBody := map[string]string{"email": "test@example.com", "password": "pass123"}
	b, _ = json.Marshal(loginBody)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login returned non-200: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("login returned success=false: %s", string(resp.Data))
	}

	var loginData struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &loginData); err != nil {
		t.Fatalf("failed to parse login data: %v", err)
	}
	if loginData.Token == "" {
		t.Fatalf("login returned empty token")
	}

	// 3) Validate token (should be valid)
	rr, err = doRequest(r, "GET", "/token/validate", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": loginData.Token})
	if err != nil {
		t.Fatalf("validate token request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("token validate returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 4) Create patient (public)
	patientBody := map[string]interface{}{
		"full_name":    "Patient One",
		"gender":       "Male",
		"age":          30,
		"job":          "Tester",
		"address":      "123 Test St",
		"email":        "patient1@example.com",
		"phone_number": []string{"081200000"},
	}
	b, _ = json.Marshal(patientBody)
	rr, err = doRequest(r, "POST", "/patient", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("create patient request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("create patient returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 5) Logout
	rr, err = doRequest(r, "DELETE", "/logout", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": loginData.Token})
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("logout returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 6) Validate token again (should fail)
	rr, err = doRequest(r, "GET", "/token/validate", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": loginData.Token})
	if err != nil {
		t.Fatalf("validate token request failed: %v", err)
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("token validate unexpectedly succeeded after logout: %s", rr.Body.String())
	}
}
