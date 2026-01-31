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
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type apiResp struct {
	Success bool            `json:"success"`
	Error   string          `json:"error"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

// requestParams groups HTTP request parameters to reduce function arguments
type requestParams struct {
	method  string
	path    string
	body    []byte
	headers map[string]string
}

func doRequest(r http.Handler, params requestParams) (*httptest.ResponseRecorder, error) {
	req, err := http.NewRequest(params.method, params.path, bytes.NewBuffer(params.body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range params.headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr, nil
}

// setupTestDB initializes database and returns connection
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

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

	t.Cleanup(func() {
		if err := db.Migrator().DropTable(testModels...); err != nil {
			t.Errorf("Failed to drop tables during cleanup: %v", err)
		}
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

	return db
}

// setupTestRouter creates and configures the Gin router
func setupTestRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))

	r.POST("/signup", endpoint.Signup)
	r.POST("/login", endpoint.Login)
	r.GET("/token/validate", endpoint.ValidateToken)
	r.POST("/patient", endpoint.CreatePatient)

	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		auth.DELETE("/logout", endpoint.Logout)
	}

	return r
}

// testSignup performs signup and returns token
func testSignup(t *testing.T, r http.Handler, apiToken string) string {
	signupBody := map[string]string{"name": "Test User", "email": "test@example.com", "password": "pass1234"}
	b, _ := json.Marshal(signupBody)
	rr, err := doRequest(r, requestParams{
		method:  "POST",
		path:    "/signup",
		body:    b,
		headers: map[string]string{"Authorization": "Bearer " + apiToken},
	})
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
	return signupToken
}

// testLogin performs login and returns session token
func testLogin(t *testing.T, r http.Handler, apiToken string) string {
	loginBody := map[string]string{"email": "test@example.com", "password": "pass1234"}
	b, _ := json.Marshal(loginBody)
	rr, err := doRequest(r, requestParams{
		method:  "POST",
		path:    "/login",
		body:    b,
		headers: map[string]string{"Authorization": "Bearer " + apiToken},
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	var resp apiResp
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
	return loginData.Token
}

// testValidateToken validates a session token
func testValidateToken(t *testing.T, r http.Handler, apiToken, sessionToken string, expectSuccess bool) {
	rr, err := doRequest(r, requestParams{
		method: "GET",
		path:   "/token/validate",
		body:   nil,
		headers: map[string]string{
			"Authorization": "Bearer " + apiToken,
			"session-token": sessionToken,
		},
	})
	if err != nil {
		t.Fatalf("validate token request failed: %v", err)
	}

	if expectSuccess && rr.Code != http.StatusOK {
		t.Fatalf("token validate returned non-200: %d %s", rr.Code, rr.Body.String())
	}
	if !expectSuccess && rr.Code == http.StatusOK {
		t.Fatalf("token validate unexpectedly succeeded: %s", rr.Body.String())
	}
}

// testCreatePatient creates a patient
func testCreatePatient(t *testing.T, r http.Handler, apiToken string) {
	patientBody := map[string]interface{}{
		"full_name":    "Patient One",
		"gender":       "Male",
		"age":          30,
		"job":          "Tester",
		"address":      "123 Test St",
		"email":        "patient1@example.com",
		"phone_number": []string{"081200000"},
	}
	b, _ := json.Marshal(patientBody)
	rr, err := doRequest(r, requestParams{
		method:  "POST",
		path:    "/patient",
		body:    b,
		headers: map[string]string{"Authorization": "Bearer " + apiToken},
	})
	if err != nil {
		t.Fatalf("create patient request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("create patient returned non-200: %d %s", rr.Code, rr.Body.String())
	}
}

// testLogout performs logout
func testLogout(t *testing.T, r http.Handler, apiToken, sessionToken string) {
	rr, err := doRequest(r, requestParams{
		method: "DELETE",
		path:   "/logout",
		body:   nil,
		headers: map[string]string{
			"Authorization": "Bearer " + apiToken,
			"session-token": sessionToken,
		},
	})
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("logout returned non-200: %d %s", rr.Code, rr.Body.String())
	}
}

func TestIntegrationFlow(t *testing.T) {
	db := setupTestDB(t)
	r := setupTestRouter(db)
	apiToken := "test-api-token"

	// 1) Signup
	testSignup(t, r, apiToken)

	// 2) Login
	sessionToken := testLogin(t, r, apiToken)

	// 3) Validate token (should be valid)
	testValidateToken(t, r, apiToken, sessionToken, true)

	// 4) Create patient (public)
	testCreatePatient(t, r, apiToken)

	// 5) Logout
	testLogout(t, r, apiToken, sessionToken)

	// 6) Validate token again (should fail)
	testValidateToken(t, r, apiToken, sessionToken, false)
}
