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

	// 13) Signup a third user for email update testing
	signupBody3 := map[string]string{"name": "Test User 3", "email": "user3@example.com", "password": "user3pass"}
	b, _ = json.Marshal(signupBody3)
	rr, err = doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup user3 failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup user3 non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 14) Login third user
	loginBody3 := map[string]string{"email": "user3@example.com", "password": "user3pass"}
	b, _ = json.Marshal(loginBody3)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login user3 failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login user3 non-200: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login3 resp failed: %v", err)
	}
	var user3Data struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &user3Data); err != nil {
		t.Fatalf("parse login3 data failed: %v", err)
	}

	// 15) Test UpdateUser with email update - should succeed with unique email
	emailUpdate := map[string]string{"email": "user3-updated@example.com"}
	b, _ = json.Marshal(emailUpdate)
	rr, err = doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": user3Data.Token})
	if err != nil {
		t.Fatalf("email update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("email update non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 16) Verify email was updated in database
	var user3Updated model.User
	if err := db.First(&user3Updated, user3Data.UserID).Error; err != nil {
		t.Fatalf("failed to query user3 after email update: %v", err)
	}
	if user3Updated.Email != "user3-updated@example.com" {
		t.Fatalf("expected email to be user3-updated@example.com; got %s", user3Updated.Email)
	}

	// 17) Test UpdateUser with duplicate email - should fail with uniqueness constraint
	duplicateEmailUpdate := map[string]string{"email": "admin@example.com"} // admin's email
	b, _ = json.Marshal(duplicateEmailUpdate)
	rr, err = doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": user3Data.Token})
	if err != nil {
		t.Fatalf("duplicate email update request failed: %v", err)
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("expected duplicate email update to fail but got 200: %s", rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode duplicate email resp failed: %v", err)
	}
	if resp.Msg != "Email already exists" {
		t.Fatalf("expected error message 'Email already exists'; got %s", resp.Msg)
	}

	// 18) Signup a fourth user for admin email update testing
	signupBody4 := map[string]string{"name": "Test User 4", "email": "user4@example.com", "password": "user4pass"}
	b, _ = json.Marshal(signupBody4)
	rr, err = doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup user4 failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup user4 non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 19) Login fourth user to get ID
	loginBody4 := map[string]string{"email": "user4@example.com", "password": "user4pass"}
	b, _ = json.Marshal(loginBody4)
	rr, err = doRequest(r, "POST", "/login", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("login user4 failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("login user4 non-200: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login4 resp failed: %v", err)
	}
	var user4Data struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal(resp.Data, &user4Data); err != nil {
		t.Fatalf("parse login4 data failed: %v", err)
	}

	// 20) Admin: UpdateUserByID with email update - should succeed with unique email
	user4Path := "/user/" + strconv.Itoa(int(user4Data.UserID))
	adminEmailUpdate := map[string]string{"email": "user4-admin-updated@example.com"}
	b, _ = json.Marshal(adminEmailUpdate)
	rr, err = doRequest(r, "PATCH", user4Path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("admin email update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("admin email update non-200: %d %s", rr.Code, rr.Body.String())
	}

	// 21) Verify email was updated in database by admin
	var user4Updated model.User
	if err := db.First(&user4Updated, user4Data.UserID).Error; err != nil {
		t.Fatalf("failed to query user4 after admin email update: %v", err)
	}
	if user4Updated.Email != "user4-admin-updated@example.com" {
		t.Fatalf("expected email to be user4-admin-updated@example.com; got %s", user4Updated.Email)
	}

	// 22) Admin: UpdateUserByID with duplicate email - should fail with uniqueness constraint
	adminDuplicateEmailUpdate := map[string]string{"email": "admin@example.com"} // admin's own email
	b, _ = json.Marshal(adminDuplicateEmailUpdate)
	rr, err = doRequest(r, "PATCH", user4Path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
	if err != nil {
		t.Fatalf("admin duplicate email update request failed: %v", err)
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("expected admin duplicate email update to fail but got 200: %s", rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode admin duplicate email resp failed: %v", err)
	}
	if resp.Msg != "Email already exists" {
		t.Fatalf("expected error message 'Email already exists'; got %s", resp.Msg)
	}
}

// TestListUsersPaginationAndSearch tests ListUsers pagination and search functionality
func TestListUsersPaginationAndSearch(t *testing.T) {
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
		userAdmin := auth.Group("/user")
		userAdmin.Use(middleware.RequireRole(model.RoleAdmin))
		{
			userAdmin.GET("", endpoint.ListUsers)
		}
	}

	// Create admin user
	signupBody := map[string]string{"name": "Admin User", "email": "admin@example.com", "password": "adminpass"}
	b, _ := json.Marshal(signupBody)
	rr, err := doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
	if err != nil {
		t.Fatalf("signup admin failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("signup admin returned non-200: %d %s", rr.Code, rr.Body.String())
	}

	// Login admin
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

	// Create additional test users
	testUsers := []map[string]string{
		{"name": "Alice Johnson", "email": "alice@example.com", "password": "pass123"},
		{"name": "Bob Smith", "email": "bob@example.com", "password": "pass123"},
		{"name": "Charlie Brown", "email": "charlie@example.com", "password": "pass123"},
		{"name": "David Wilson", "email": "david@example.com", "password": "pass123"},
		{"name": "Eve Davis", "email": "eve@example.com", "password": "pass123"},
	}

	for _, user := range testUsers {
		b, _ := json.Marshal(user)
		rr, err := doRequest(r, "POST", "/signup", b, map[string]string{"Authorization": "Bearer test-api-token"})
		if err != nil {
			t.Fatalf("signup %s failed: %v", user["email"], err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("signup %s returned non-200: %d %s", user["email"], rr.Code, rr.Body.String())
		}
	}

	// Test 1: List all users without pagination
	t.Run("ListAllUsersNoPagination", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		total := int(data["total"].(float64))
		if total != 6 { // Admin + 5 test users
			t.Errorf("expected 6 total users, got %d", total)
		}
	})

	// Test 2: List users with limit parameter
	t.Run("ListUsersWithLimit", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?limit=3", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with limit failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with limit returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 3 {
			t.Errorf("expected 3 fetched users, got %d", totalFetched)
		}
	})

	// Test 3: List users with offset parameter
	t.Run("ListUsersWithOffset", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?offset=2", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with offset failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with offset returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 4 { // Should skip first 2 users, return remaining 4
			t.Errorf("expected 4 fetched users, got %d", totalFetched)
		}
	})

	// Test 4: List users with both limit and offset
	t.Run("ListUsersWithLimitAndOffset", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?limit=2&offset=1", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with limit and offset failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with limit and offset returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 2 {
			t.Errorf("expected 2 fetched users, got %d", totalFetched)
		}
	})

	// Test 5: List users with keyword search
	t.Run("ListUsersWithKeywordSearch", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?keyword=alice", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with keyword failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with keyword returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		total := int(data["total"].(float64))
		if total != 1 { // Only Alice Johnson should match
			t.Errorf("expected 1 user matching 'alice', got %d", total)
		}
	})

	// Test 6: List users with keyword search by email
	t.Run("ListUsersWithKeywordSearchByEmail", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?keyword=bob@example", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with email keyword failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with email keyword returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		total := int(data["total"].(float64))
		if total != 1 { // Only Bob Smith should match
			t.Errorf("expected 1 user matching 'bob@example', got %d", total)
		}
	})

	// Test 7: List users with keyword and pagination
	t.Run("ListUsersWithKeywordAndPagination", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?keyword=example&limit=2&offset=1", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with keyword and pagination failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with keyword and pagination returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 2 {
			t.Errorf("expected 2 fetched users, got %d", totalFetched)
		}

		total := int(data["total"].(float64))
		if total != 6 { // All users have 'example' in their email
			t.Errorf("expected 6 total users matching 'example', got %d", total)
		}
	})

	// Test 8: Edge case - negative limit (should be ignored)
	t.Run("ListUsersWithNegativeLimit", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?limit=-5", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with negative limit failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with negative limit returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		// Negative limit should be ignored, all users should be returned
		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 6 {
			t.Errorf("expected all 6 users (negative limit ignored), got %d", totalFetched)
		}
	})

	// Test 9: Edge case - negative offset (should be ignored)
	t.Run("ListUsersWithNegativeOffset", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?offset=-3", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with negative offset failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with negative offset returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		// Negative offset should be ignored, all users should be returned
		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 6 {
			t.Errorf("expected all 6 users (negative offset ignored), got %d", totalFetched)
		}
	})

	// Test 10: Edge case - very large limit
	t.Run("ListUsersWithVeryLargeLimit", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?limit=10000", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with large limit failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with large limit returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		// Should return all available users (6 in total)
		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 6 {
			t.Errorf("expected 6 users (all available), got %d", totalFetched)
		}
	})

	// Test 11: Edge case - empty keyword search (should return all users)
	t.Run("ListUsersWithEmptyKeyword", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?keyword=", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with empty keyword failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with empty keyword returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		// Empty keyword should return all users
		total := int(data["total"].(float64))
		if total != 6 {
			t.Errorf("expected 6 users (empty keyword returns all), got %d", total)
		}
	})

	// Test 12: Edge case - keyword with no matches
	t.Run("ListUsersWithKeywordNoMatches", func(t *testing.T) {
		rr, err := doRequest(r, "GET", "/user?keyword=nonexistent", nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminData.Token})
		if err != nil {
			t.Fatalf("list users with no-match keyword failed: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("list users with no-match keyword returned non-200: %d %s", rr.Code, rr.Body.String())
		}

		var resp apiResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}

		// No matches should return 0 users
		total := int(data["total"].(float64))
		if total != 0 {
			t.Errorf("expected 0 users (no matches), got %d", total)
		}

		totalFetched := int(data["total_fetched"].(float64))
		if totalFetched != 0 {
			t.Errorf("expected 0 fetched users (no matches), got %d", totalFetched)
		}
	})
}
