package endpoint_test

import (
	"encoding/json"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
)

// loginMustChangePassword creates a user with the given password, logs in
// and returns the must_change_password flag from the login response.
// The user is inserted directly because signup enforces a minimum length
// that the default password does not meet.
func loginMustChangePassword(t *testing.T, email, password string) bool {
	t.Helper()
	db := setupTestDB(t)
	r := setupTestRouter(db)

	salt, err := util.GenerateSalt()
	if err != nil {
		t.Fatalf("generate salt: %v", err)
	}
	hash, err := util.HashPasswordArgon2(password, salt)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := model.User{Name: "Test User", Email: email, Password: hash, PasswordSalt: salt, RoleID: 1}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": password})
	rr := performRequest(t, r, requestParams{method: "POST", path: "/login", body: loginBody}, requestOpts{})
	data := validateAndDecodeResponse(t, rr, "POST", "/login")

	var loginData map[string]interface{}
	if err := json.Unmarshal(data, &loginData); err != nil {
		t.Fatalf("failed to parse login data: %v", err)
	}
	flag, present := loginData["must_change_password"]
	if !present {
		t.Fatalf("login response missing must_change_password: %s", string(data))
	}
	b, ok := flag.(bool)
	if !ok {
		t.Fatalf("must_change_password is not a bool: %v", flag)
	}
	return b
}

func TestLoginFlagsDefaultPassword(t *testing.T) {
	t.Setenv("DEFAULT_USER_PASSWORD", "")
	if !loginMustChangePassword(t, "default@example.com", "LTT2025") {
		t.Fatal("expected must_change_password=true when logging in with the default password")
	}
}

func TestLoginDoesNotFlagCustomPassword(t *testing.T) {
	t.Setenv("DEFAULT_USER_PASSWORD", "")
	if loginMustChangePassword(t, "custom@example.com", "pass1234") {
		t.Fatal("expected must_change_password=false for a non-default password")
	}
}

func TestLoginFlagsConfiguredDefaultPassword(t *testing.T) {
	t.Setenv("DEFAULT_USER_PASSWORD", "Welcome123")
	if !loginMustChangePassword(t, "configured@example.com", "Welcome123") {
		t.Fatal("expected must_change_password=true for the configured default password")
	}
}

func TestLoginDoesNotFlagOldDefaultWhenConfigured(t *testing.T) {
	t.Setenv("DEFAULT_USER_PASSWORD", "Welcome123")
	if loginMustChangePassword(t, "legacy@example.com", "LTT2025") {
		t.Fatal("expected must_change_password=false for LTT2025 when another default is configured")
	}
}
