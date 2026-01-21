package endpoint_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/endpoint"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

// This test covers admin user management and self-profile update flows.
func TestUserEndpoints(t *testing.T) {
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret-123")
	t.Setenv("APITOKEN", "test-api-token")
	t.Setenv("GINMODE", "release")

	util.SetJWTSecret("test-secret-123")

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

	testModels := []interface{}{
		&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.PatientCode{},
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

	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))

	// Public routes
	r.POST("/signup", endpoint.Signup)
	r.POST("/login", endpoint.Login)
	r.GET("/token/validate", endpoint.ValidateToken)

	// Protected routes
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		auth.DELETE("/logout", endpoint.Logout)
		auth.PATCH("/user", endpoint.UpdateUser)

		userAdmin := auth.Group("/user")
		userAdmin.Use(middleware.RequireRole(model.RoleAdmin))
		{
			userAdmin.GET("", endpoint.ListUsers)
			userAdmin.GET("/:id", endpoint.GetUserInfo)
			userAdmin.PATCH("/:id", endpoint.UpdateUserByID)
			userAdmin.DELETE("/:id", endpoint.DeleteUser)
		}
	}

	// helper to perform requests using shared doRequest from integration_test.go

	// 1) Signup admin user
	signupBody := map[string]string{"name": "Admin User", "email": "admin@example.com", "password": "adminpass"}
	b, _ := json.Marshal(signupBody)
	rr, err := doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup admin failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup admin returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	var resp apiResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode signup resp failed: %v", err)
	}
	var _signupToken string
	if err := json.Unmarshal(resp.Data, &_signupToken); err != nil {
		t.Fatalf("parse signup token failed: %v", err)
	}

	// 2) Login admin
	loginBody := map[string]string{"email": "admin@example.com", "password": "adminpass"}
	b, _ = json.Marshal(loginBody)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login admin failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login admin non-200: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login resp failed: %v", err)
	}
	var adminData struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &adminData); err != nil {
		t.Fatalf("parse login data failed: %v", err)
	}
	if adminData.Token == "" {
		t.Fatalf("admin login returned empty token")
	}

	// 3) Signup target user
	signupBody2 := map[string]string{"name": "Target User", "email": "target@example.com", "password": "targetpass"}
	b, _ = json.Marshal(signupBody2)
	rr, err = doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup target failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup target non-200: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode signup2 resp failed: %v", err)
	}
	var _signupToken2 string
	if err := json.Unmarshal(resp.Data, &_signupToken2); err != nil {
		t.Fatalf("parse signup2 token failed: %v", err)
	}

	// 4) Login target to get ID
	loginBody2 := map[string]string{"email": "target@example.com", "password": "targetpass"}
	b, _ = json.Marshal(loginBody2)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login target failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login target non-200: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login2 resp failed: %v", err)
	}
	var targetData struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &targetData); err != nil {
		t.Fatalf("parse login2 data failed: %v", err)
	}

	// 5) Admin: ListUsers
	rr, err = doRequest(r, "GET", "/user", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("list users failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("list users returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 6) Admin: GetUserInfo for target
	path := "/user/" + strconv.Itoa(int(targetData.UserID))
	rr, err = doRequest(r, "GET", path, nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("get user info failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("get user info non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 7) Admin: UpdateUserByID (change password)
	updateBody := map[string]string{"password": "newtargetpass", "name": "Target Updated"}
	b, _ = json.Marshal(updateBody)
	rr, err = doRequest(r, "PATCH", path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("admin update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("admin update non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 8) Verify in DB that target's password was updated to the new hashed value
	var targetUser model.User
	if err := db.First(&targetUser, targetData.UserID).Error; err != nil {
		t.Fatalf("failed to query target user from DB: %v", err)
	}
	expectedHash := util.HashPassword("newtargetpass")
	if targetUser.Password != expectedHash {
		t.Fatalf("expected target password hash to be updated; got %s, want %s", targetUser.Password, expectedHash)
	}

	// 9) Target (self) updates own profile using their existing session token
	selfUpdate := map[string]string{"name": "Target Self Updated", "password": "finalpass"}
	b, _ = json.Marshal(selfUpdate)
	rr, err = doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": targetData.Token})
	if err != nil {
		t.Fatalf("self update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("self update non-200: %d %s", rr.Code, rr.Body.String())
	}
	// 10) Verify in DB that target's password was updated to the final hashed value
	if err := db.First(&targetUser, targetData.UserID).Error; err != nil {
		t.Fatalf("failed to query target user after self-update: %v", err)
	}
	expectedFinalHash := util.HashPassword("finalpass")
	if targetUser.Password != expectedFinalHash {
		t.Fatalf("expected target password hash to be finalpass; got %s, want %s", targetUser.Password, expectedFinalHash)
	}

	// 11) Admin: DeleteUser target
	rr, err = doRequest(r, "DELETE", path, nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("delete user failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("delete user non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 12) Admin: GetUserInfo should now return 404
	rr, err = doRequest(r, "GET", path, nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("get user after delete failed: %v", err)
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("expecting not found after delete but got 200: %s", rr.Body.String())
	}
}

// TestUpdateUserValidation tests that UpdateUser and AdminUpdateUser validate at least one field is provided
func TestUpdateUserValidation(t *testing.T) {
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret-123")
	t.Setenv("APITOKEN", "test-api-token")
	t.Setenv("GINMODE", "release")

	util.SetJWTSecret("test-secret-123")

	cfg := config.LoadConfig()
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

	testModels := []interface{}{
		&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.PatientCode{},
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

	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))

	// Public routes
	r.POST("/signup", endpoint.Signup)
	r.POST("/login", endpoint.Login)

	// Protected routes
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		auth.PATCH("/user", endpoint.UpdateUser)

		userAdmin := auth.Group("/user")
		userAdmin.Use(middleware.RequireRole(model.RoleAdmin))
		{
			userAdmin.PATCH("/:id", endpoint.UpdateUserByID)
		}
	}

	// 1) Signup admin user
	signupBody := map[string]string{"name": "Admin User", "email": "admin@example.com", "password": "adminpass"}
	b, _ := json.Marshal(signupBody)
	rr, err := doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup admin failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup admin returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 2) Login admin
	loginBody := map[string]string{"email": "admin@example.com", "password": "adminpass"}
	b, _ = json.Marshal(loginBody)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login admin failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login admin non-200: %d %s", rr.Code, rr.Body.String())
	}
	var resp apiResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login resp failed: %v", err)
	}
	var adminData struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &adminData); err != nil {
		t.Fatalf("parse login data failed: %v", err)
	}

	// 3) Signup target user
	signupBody2 := map[string]string{"name": "Target User", "email": "target@example.com", "password": "targetpass"}
	b, _ = json.Marshal(signupBody2)
	rr, err = doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup target failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup target non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 4) Login target to get token
	loginBody2 := map[string]string{"email": "target@example.com", "password": "targetpass"}
	b, _ = json.Marshal(loginBody2)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login target failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login target non-200: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login2 resp failed: %v", err)
	}
	var targetData struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &targetData); err != nil {
		t.Fatalf("parse login2 data failed: %v", err)
	}

	// 5) Test self-update with empty payload - should return 400
	emptyUpdate := map[string]string{}
	b, _ = json.Marshal(emptyUpdate)
	rr, err = doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": targetData.Token})
	if err != nil {
		t.Fatalf("empty self update request failed: %v", err)
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty update, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode empty update resp failed: %v", err)
	}
	expectedMsg := "At least one field (name, email, or password) must be provided"
	if resp.Msg != expectedMsg {
		t.Fatalf("expected error message '%s', got '%s'", expectedMsg, resp.Msg)
	}

	// 6) Test admin update with empty payload - should return 400
	path := "/user/" + strconv.Itoa(int(targetData.UserID))
	rr, err = doRequest(r, "PATCH", path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("empty admin update request failed: %v", err)
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty admin update, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode empty admin update resp failed: %v", err)
	}
	if resp.Msg != expectedMsg {
		t.Fatalf("expected error message '%s', got '%s'", expectedMsg, resp.Msg)
	}

	// 7) Test self-update with at least one field - should succeed
	validUpdate := map[string]string{"name": "Updated Name"}
	b, _ = json.Marshal(validUpdate)
	rr, err = doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": targetData.Token})
	if err != nil {
		t.Fatalf("valid self update request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid update, got %d: %s", rr.Code, rr.Body.String())
	}

	// 8) Test admin update with at least one field - should succeed
	validUpdate2 := map[string]string{"email": "newemail@example.com"}
	b, _ = json.Marshal(validUpdate2)
	rr, err = doRequest(r, "PATCH", path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("valid admin update request failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid admin update, got %d: %s", rr.Code, rr.Body.String())
	}
}
