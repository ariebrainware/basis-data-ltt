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

// requestOpts groups optional parameters for HTTP requests
type requestOpts struct {
	apiToken     string
	sessionToken string
}

// performRequest executes an HTTP request and validates response status
func performRequest(t *testing.T, r http.Handler, params requestParams, opts requestOpts) *httptest.ResponseRecorder {
	headers := map[string]string{}
	if opts.apiToken != "" {
		headers["Authorization"] = "Bearer " + opts.apiToken
	}
	if opts.sessionToken != "" {
		headers["session-token"] = opts.sessionToken
	}

	// merge provided headers
	if params.headers == nil {
		params.headers = headers
	} else {
		for k, v := range headers {
			params.headers[k] = v
		}
	}

	rr, err := doRequest(r, params)
	if err != nil {
		t.Fatalf("%s %s request failed: %v", params.method, params.path, err)
	}
	return rr
}

// validateAndDecodeResponse checks response success and decodes data
func validateAndDecodeResponse(t *testing.T, rr *httptest.ResponseRecorder, method, path string) json.RawMessage {
	if rr.Code != http.StatusOK {
		t.Fatalf("%s %s returned non-200: %d %s", method, path, rr.Code, rr.Body.String())
	}

	var resp apiResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode %s %s response: %v", method, path, err)
	}
	if !resp.Success {
		t.Fatalf("%s %s returned success=false: %s", method, path, string(resp.Data))
	}
	return resp.Data
}

// testSignup performs signup and returns token
func testSignup(t *testing.T, r http.Handler, apiToken string) string {
	signupBody := map[string]string{"name": "Test User", "email": "test@example.com", "password": "pass1234"}
	b, _ := json.Marshal(signupBody)

	rr := performRequest(t, r, requestParams{method: "POST", path: "/signup", body: b}, requestOpts{apiToken: apiToken})
	data := validateAndDecodeResponse(t, rr, "POST", "/signup")

	var signupToken string
	if err := json.Unmarshal(data, &signupToken); err != nil {
		t.Fatalf("failed to parse signup token: %v", err)
	}
	return signupToken
}

// testLogin performs login and returns session token
func testLogin(t *testing.T, r http.Handler, apiToken string) string {
	loginBody := map[string]string{"email": "test@example.com", "password": "pass1234"}
	b, _ := json.Marshal(loginBody)

	rr := performRequest(t, r, requestParams{method: "POST", path: "/login", body: b}, requestOpts{apiToken: apiToken})
	data := validateAndDecodeResponse(t, rr, "POST", "/login")

	var loginData struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(data, &loginData); err != nil {
		t.Fatalf("failed to parse login data: %v", err)
	}
	if loginData.Token == "" {
		t.Fatalf("login returned empty token")
	}
	return loginData.Token
}

// validateTokenOpts groups parameters for token validation
type validateTokenOpts struct {
	apiToken      string
	sessionToken  string
	expectSuccess bool
}

// testValidateToken validates a session token
func testValidateToken(t *testing.T, r http.Handler, opts validateTokenOpts) {
	rr := performRequest(t, r, requestParams{method: "GET", path: "/token/validate"}, requestOpts{
		apiToken:     opts.apiToken,
		sessionToken: opts.sessionToken,
	})

	if opts.expectSuccess && rr.Code != http.StatusOK {
		t.Fatalf("token validate returned non-200: %d %s", rr.Code, rr.Body.String())
	}
	if !opts.expectSuccess && rr.Code == http.StatusOK {
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
	_ = performRequest(t, r, requestParams{method: "POST", path: "/patient", body: b}, requestOpts{apiToken: apiToken})
}

// testLogout performs logout
func testLogout(t *testing.T, r http.Handler, opts requestOpts) {
	_ = performRequest(t, r, requestParams{method: "DELETE", path: "/logout"}, opts)
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
	testValidateToken(t, r, validateTokenOpts{
		apiToken:      apiToken,
		sessionToken:  sessionToken,
		expectSuccess: true,
	})

	// 4) Create patient (public)
	testCreatePatient(t, r, apiToken)

	// 5) Logout
	testLogout(t, r, requestOpts{
		apiToken:     apiToken,
		sessionToken: sessionToken,
	})

	// 6) Validate token again (should fail)
	testValidateToken(t, r, validateTokenOpts{
		apiToken:      apiToken,
		sessionToken:  sessionToken,
		expectSuccess: false,
	})
}
