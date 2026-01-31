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
