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

// reuse apiResp defined in integration_test.go

// Unit test: calling VerifyPassword without an authenticated user should return 401
func TestVerifyPasswordUnauthorized(t *testing.T) {
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "unit-secret")
	t.Setenv("APITOKEN", "test-api-token")

	util.SetJWTSecret("unit-secret")

	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

	// Ensure minimal migration
	if err := db.AutoMigrate(&model.User{}, &model.Session{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "/verify-password?password=whatever", nil)
	c.Request = req
	c.Set(middleware.DBKey, db)

	endpoint.VerifyPassword(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when user not authenticated, got %d body=%s", w.Code, w.Body.String())
	}
}

// Integration test: signup/login then verify password (correct and incorrect)
func TestVerifyPasswordIntegration(t *testing.T) {
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "integ-secret-123")
	t.Setenv("APITOKEN", "test-api-token")
	t.Setenv("GINMODE", "release")

	util.SetJWTSecret("integ-secret-123")

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

	testModels := []interface{}{&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.PatientCode{}}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(testModels...)
	})

	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seeding roles failed: %v", err)
	}
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

	// Protected verify-password endpoint
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		auth.GET("/verify-password", endpoint.VerifyPassword)
	}

	// Signup
	signupBody := map[string]string{"name": "Test User", "email": "vp@example.com", "password": "pass123"}
	b, _ := json.Marshal(signupBody)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/signup", bytesNewBuffer(b))
	req.Header.Set("Authorization", "Bearer test-api-token")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("signup failed: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp apiResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode signup response: %v", err)
	}

	// Login
	loginBody := map[string]string{"email": "vp@example.com", "password": "pass123"}
	b, _ = json.Marshal(loginBody)
	rr = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/login", bytesNewBuffer(b))
	req.Header.Set("Authorization", "Bearer test-api-token")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
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

	// Correct password should verify
	rr = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/verify-password?password=pass123", nil)
	req.Header.Set("Authorization", "Bearer test-api-token")
	req.Header.Set("session-token", loginData.Token)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("verify-password (correct) failed: %d body=%s", rr.Code, rr.Body.String())
	}

	// Incorrect password should be unauthorized
	rr = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/verify-password?password=wrongpass", nil)
	req.Header.Set("Authorization", "Bearer test-api-token")
	req.Header.Set("session-token", loginData.Token)
	r.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("verify-password unexpectedly succeeded with wrong password: body=%s", rr.Body.String())
	}
}

// tiny helper to avoid importing bytes repeatedly
func bytesNewBuffer(b []byte) *bytes.Buffer {
	return bytes.NewBuffer(b)
}
