package endpoint_test

import (
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

// SetupTestServer initializes DB, migrates models, seeds roles and returns a Gin router
// and a cleanup function that drops tables. It calls t.Fatalf on fatal errors.
func SetupTestServer(t *testing.T) (*gin.Engine, *gorm.DB, func()) {
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

	testModels := []interface{}{
		&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.PatientCode{},
	}

	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	if err := model.SeedRoles(db); err != nil {
		t.Fatalf("seeding roles failed: %v", err)
	}

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))

	// Public routes used by tests
	r.POST("/signup", endpoint.Signup)
	r.POST("/login", endpoint.Login)
	r.GET("/token/validate", endpoint.ValidateToken)

	// Protected routes used by tests
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		auth.DELETE("/logout", endpoint.Logout)
		auth.PATCH("/user", endpoint.UpdateUser)

		userAdmin := auth.Group("/user")
		userAdmin.Use(middleware.RequireRole(model.RoleAdmin))
		{
			userAdmin.GET("", endpoint.ListUsers)
			userAdmin.GET(":id", endpoint.GetUserInfo)
			userAdmin.PATCH(":id", endpoint.UpdateUserByID)
			userAdmin.DELETE(":id", endpoint.DeleteUser)
		}
	}

	cleanup := func() {
		if err := db.Migrator().DropTable(testModels...); err != nil {
			t.Errorf("failed to drop tables during cleanup: %v", err)
		}
	}

	return r, db, cleanup
}

// CreateAndLoginUser signs up and logs in a user, returning session token and user id.
// It fails the test on error.
type SignupCreds struct {
	Name     string
	Email    string
	Password string
}

func CreateAndLoginUser(t *testing.T, r http.Handler, creds SignupCreds) (string, uint) {
	signupBody := map[string]string{"name": creds.Name, "email": creds.Email, "password": creds.Password}
	b, _ := json.Marshal(signupBody)
	rr, err := doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup %s failed: %v", creds.Email, err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup %s returned non-200: %d %s", creds.Email, rr.Code, rr.Body.String())
	}

	// Login
	loginBody := map[string]string{"email": creds.Email, "password": creds.Password}
	b, _ = json.Marshal(loginBody)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login %s failed: %v", creds.Email, err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login %s returned non-200: %d %s", creds.Email, rr.Code, rr.Body.String())
	}

	var resp apiResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login resp failed: %v", err)
	}
	var data struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("parse login data failed: %v", err)
	}
	return data.Token, data.UserID
}

// SetupServerWithUser initializes the server and returns a logged-in user session.
func SetupServerWithUser(t *testing.T, creds SignupCreds) (*gin.Engine, *gorm.DB, string, uint) {
	r, db, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	token, userID := CreateAndLoginUser(t, r, creds)
	return r, db, token, userID
}

// SetupServerWithAdmin initializes the server and returns a logged-in admin session.
func SetupServerWithAdmin(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	r, db, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	adminToken, _ := CreateAndLoginUser(t, r, SignupCreds{Name: "Admin User", Email: "admin@example.com", Password: "adminpass"})
	return r, db, adminToken
}

// SetupServerWithAdminAndUser initializes the server and returns admin and user sessions.
func SetupServerWithAdminAndUser(t *testing.T, userCreds SignupCreds) (*gin.Engine, *gorm.DB, string, string, uint) {
	r, db, adminToken := SetupServerWithAdmin(t)

	userToken, userID := CreateAndLoginUser(t, r, userCreds)
	return r, db, adminToken, userToken, userID
}

// ParseAPIResp decodes a standard API response from a ResponseRecorder.
// It fails the test on decoding error.
func ParseAPIResp(t *testing.T, rr *httptest.ResponseRecorder) apiResp {
	var resp apiResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v; body: %s", err, rr.Body.String())
	}
	return resp
}

// ParseDataToMap unmarshals an API response Data field into a map[string]interface{}.
// It fails the test on error.
func ParseDataToMap(t *testing.T, raw json.RawMessage) map[string]interface{} {
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse data failed: %v", err)
	}
	return data
}

// CreateAdminAndTestUsers creates an admin user (logged in) and several test users.
// Returns admin session token and admin user id.
func CreateAdminAndTestUsers(t *testing.T, r http.Handler) (string, uint) {
	adminToken, adminID := CreateAndLoginUser(t, r, SignupCreds{Name: "Admin User", Email: "admin@example.com", Password: "adminpass"})

	testUsers := []map[string]string{
		{"name": "Alice Johnson", "email": "alice@example.com", "password": "pass1234"},
		{"name": "Bob Smith", "email": "bob@example.com", "password": "pass1234"},
		{"name": "Charlie Brown", "email": "charlie@example.com", "password": "pass1234"},
		{"name": "David Wilson", "email": "david@example.com", "password": "pass1234"},
		{"name": "Eve Davis", "email": "eve@example.com", "password": "pass1234"},
	}

	for _, u := range testUsers {
		b, _ := json.Marshal(u)
		rr, err := doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
		if err != nil {
			t.Fatalf("signup %s failed: %v", u["email"], err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("signup %s returned non-200: %d %s", u["email"], rr.Code, rr.Body.String())
		}
	}

	return adminToken, adminID
}

// ListUsersData performs a GET /user request with optional query string and
// returns the decoded response data as map[string]interface{}.
func ListUsersData(t *testing.T, r http.Handler, adminToken string, query string) map[string]interface{} {
	path := "/user"
	if query != "" {
		path += "?" + query
	}
	rr, err := doRequest(r, "GET", path, nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminToken})
	if err != nil {
		t.Fatalf("list users request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("list users returned non-200: %d %s", rr.Code, rr.Body.String())
	}
	resp := ParseAPIResp(t, rr)
	return ParseDataToMap(t, resp.Data)
}

// AssertTotal asserts the `total` field in list response data.
func AssertTotal(t *testing.T, data map[string]interface{}, want int) {
	got := int(data["total"].(float64))
	if got != want {
		t.Errorf("expected total %d, got %d", want, got)
	}
}

// AssertTotalFetched asserts the `total_fetched` field in list response data.
func AssertTotalFetched(t *testing.T, data map[string]interface{}, want int) {
	got := int(data["total_fetched"].(float64))
	if got != want {
		t.Errorf("expected total_fetched %d, got %d", want, got)
	}
}

// EmailUpdateRequest groups parameters for email update requests.
type EmailUpdateRequest struct {
	Token string // Session token
	Path  string // Endpoint path (defaults to /user if empty)
	Email string // New email address
}

// PatchUserEmail sends a PATCH request to update a user's email.
func PatchUserEmail(t *testing.T, r http.Handler, req EmailUpdateRequest) *httptest.ResponseRecorder {
	path := req.Path
	if path == "" {
		path = "/user"
	}
	updateBody := map[string]string{"email": req.Email}
	b, _ := json.Marshal(updateBody)
	rr, err := doRequest(r, "PATCH", path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": req.Token})
	if err != nil {
		t.Fatalf("email update failed: %v", err)
	}
	return rr
}

// AssertUserEmail verifies the user's email in the database.
func AssertUserEmail(t *testing.T, db *gorm.DB, userID uint, want string) {
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if user.Email != want {
		t.Fatalf("expected email to be %s; got %s", want, user.Email)
	}
}
