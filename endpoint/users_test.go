package endpoint_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
)

// Admin updates another user's password
func TestAdminUpdateTargetPassword(t *testing.T) {
	r, db, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	adminToken, _ := CreateAndLoginUser(t, r, SignupCreds{Name: "Admin User", Email: "admin@example.com", Password: "adminpass"})
	_, targetID := CreateAndLoginUser(t, r, SignupCreds{Name: "Target User", Email: "target@example.com", Password: "targetpass"})

	path := "/user/" + strconv.Itoa(int(targetID))
	updateBody := map[string]string{"password": "newtargetpass", "name": "Target Updated"}
	b, _ := json.Marshal(updateBody)
	rr, err := doRequest(r, "PATCH", path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminToken})
	if err != nil {
		t.Fatalf("admin update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("admin update non-200: %d %s", rr.Code, rr.Body.String())
	}

	var targetUser model.User
	if err := db.First(&targetUser, targetID).Error; err != nil {
		t.Fatalf("failed to query target user from DB: %v", err)
	}
	ok, err := util.VerifyPassword("newtargetpass", targetUser.Password, targetUser.PasswordSalt)
	if err != nil || !ok {
		t.Fatalf("updated password did not verify for target user: %v", err)
	}
}

// Admin deletes another user
func TestAdminDeleteTarget(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	adminToken, _ := CreateAndLoginUser(t, r, SignupCreds{Name: "Admin User", Email: "admin@example.com", Password: "adminpass"})
	_, targetID := CreateAndLoginUser(t, r, SignupCreds{Name: "Target User", Email: "target@example.com", Password: "targetpass"})

	path := "/user/" + strconv.Itoa(int(targetID))
	rr, err := doRequest(r, "DELETE", path, nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminToken})
	if err != nil {
		t.Fatalf("delete user failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("delete user non-200: %d %s", rr.Code, rr.Body.String())
	}

	rr, err = doRequest(r, "GET", path, nil, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminToken})
	if err != nil {
		t.Fatalf("get user after delete failed: %v", err)
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("expecting not found after delete but got 200: %s", rr.Body.String())
	}
}

func TestSelfPasswordUpdate(t *testing.T) {
	r, db, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	userToken, userID := CreateAndLoginUser(t, r, SignupCreds{Name: "Self User", Email: "self@example.com", Password: "initialpass"})

	// Self update using userToken (use endpoint /user)
	selfUpdate := map[string]string{"name": "Self Updated", "password": "finalpass"}
	b, _ := json.Marshal(selfUpdate)
	rr, err := doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": userToken})
	if err != nil {
		t.Fatalf("self update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("self update non-200: %d %s", rr.Code, rr.Body.String())
	}

	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		t.Fatalf("failed to query user after self-update: %v", err)
	}
	ok, err := util.VerifyPassword("finalpass", user.Password, user.PasswordSalt)
	if err != nil || !ok {
		t.Fatalf("final password did not verify for user: %v", err)
	}
}

func TestUserEmailUpdate(t *testing.T) {
	r, db, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	userToken, userID := CreateAndLoginUser(t, r, SignupCreds{Name: "Test User 3", Email: "user3@example.com", Password: "user3pass"})

	emailUpdate := map[string]string{"email": "user3-updated@example.com"}
	b, _ := json.Marshal(emailUpdate)
	rr, err := doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": userToken})
	if err != nil {
		t.Fatalf("email update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("email update non-200: %d %s", rr.Code, rr.Body.String())
	}

	var user3 model.User
	if err := db.First(&user3, userID).Error; err != nil {
		t.Fatalf("failed to query user3 after email update: %v", err)
	}
	if user3.Email != "user3-updated@example.com" {
		t.Fatalf("expected email to be user3-updated@example.com; got %s", user3.Email)
	}
}

func TestDuplicateEmailUpdateFails(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	adminToken, _ := CreateAndLoginUser(t, r, SignupCreds{Name: "Admin User", Email: "admin@example.com", Password: "adminpass"})
	userToken, _ := CreateAndLoginUser(t, r, SignupCreds{Name: "Test User 3", Email: "user3@example.com", Password: "user3pass"})

	duplicateEmailUpdate := map[string]string{"email": "admin@example.com"}
	b, _ := json.Marshal(duplicateEmailUpdate)
	rr, err := doRequest(r, "PATCH", "/user", b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": userToken})
	if err != nil {
		t.Fatalf("duplicate email update request failed: %v", err)
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("expected duplicate email update to fail but got 200: %s", rr.Body.String())
	}
	resp := ParseAPIResp(t, rr)
	if resp.Msg != "Email already exists" {
		t.Fatalf("expected error message 'Email already exists'; got %s", resp.Msg)
	}

	// ensure admin exists for later tests
	_ = adminToken
}

func TestAdminEmailUpdate(t *testing.T) {
	r, db, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	adminToken, _ := CreateAndLoginUser(t, r, SignupCreds{Name: "Admin User", Email: "admin@example.com", Password: "adminpass"})
	_, user4ID := CreateAndLoginUser(t, r, SignupCreds{Name: "Test User 4", Email: "user4@example.com", Password: "user4pass"})
	user4Path := "/user/" + strconv.Itoa(int(user4ID))

	adminEmailUpdate := map[string]string{"email": "user4-admin-updated@example.com"}
	b, _ := json.Marshal(adminEmailUpdate)
	rr, err := doRequest(r, "PATCH", user4Path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminToken})
	if err != nil {
		t.Fatalf("admin email update failed: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("admin email update non-200: %d %s", rr.Code, rr.Body.String())
	}

	var user4 model.User
	if err := db.First(&user4, user4ID).Error; err != nil {
		t.Fatalf("failed to query user4 after admin email update: %v", err)
	}
	if user4.Email != "user4-admin-updated@example.com" {
		t.Fatalf("expected email to be user4-admin-updated@example.com; got %s", user4.Email)
	}
}

func TestAdminDuplicateEmailUpdateFails(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)

	adminToken, _ := CreateAndLoginUser(t, r, SignupCreds{Name: "Admin User", Email: "admin@example.com", Password: "adminpass"})
	_, user4ID := CreateAndLoginUser(t, r, SignupCreds{Name: "Test User 4", Email: "user4@example.com", Password: "user4pass"})
	user4Path := "/user/" + strconv.Itoa(int(user4ID))

	adminDuplicateEmailUpdate := map[string]string{"email": "admin@example.com"}
	b, _ := json.Marshal(adminDuplicateEmailUpdate)
	rr, err := doRequest(r, "PATCH", user4Path, b, map[string]string{"Authorization": "Bearer test-api-token", "session-token": adminToken})
	if err != nil {
		t.Fatalf("admin duplicate email update request failed: %v", err)
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("expected admin duplicate email update to fail but got 200: %s", rr.Body.String())
	}
	resp := ParseAPIResp(t, rr)
	if resp.Msg != "Email already exists" {
		t.Fatalf("expected error message 'Email already exists'; got %s", resp.Msg)
	}
}

// TestListUsersPaginationAndSearch tests ListUsers pagination and search functionality
// Split the large list test into focused tests to improve readability and maintainability.
func TestListUsersNoPagination(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "")
	AssertTotal(t, data, 6)
}

func TestListUsersWithLimit(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "limit=3")
	AssertTotalFetched(t, data, 3)
}

func TestListUsersWithOffset(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "offset=2")
	AssertTotalFetched(t, data, 4)
}

func TestListUsersWithLimitAndOffset(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "limit=2&offset=1")
	AssertTotalFetched(t, data, 2)
}

func TestListUsersWithKeywordSearch(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "keyword=alice")
	AssertTotal(t, data, 1)
}

func TestListUsersWithKeywordSearchByEmail(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "keyword=bob@example")
	AssertTotal(t, data, 1)
}

func TestListUsersWithKeywordAndPagination(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "keyword=example&limit=2&offset=1")
	AssertTotalFetched(t, data, 2)
	AssertTotal(t, data, 6)
}

func TestListUsersWithNegativeLimit(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "limit=-5")
	AssertTotalFetched(t, data, 6)
}

func TestListUsersWithNegativeOffset(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "offset=-3")
	AssertTotalFetched(t, data, 6)
}

func TestListUsersWithVeryLargeLimit(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "limit=10000")
	AssertTotalFetched(t, data, 6)
}

func TestListUsersWithEmptyKeyword(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "keyword=")
	AssertTotal(t, data, 6)
}

func TestListUsersWithKeywordNoMatches(t *testing.T) {
	r, _, cleanup := SetupTestServer(t)
	t.Cleanup(cleanup)
	adminToken, _ := CreateAdminAndTestUsers(t, r)

	data := ListUsersData(t, r, adminToken, "keyword=nonexistent")
	AssertTotal(t, data, 0)
	AssertTotalFetched(t, data, 0)
}
