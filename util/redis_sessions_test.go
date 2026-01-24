package util

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
)

func TestAddSessionToUserSet_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	token := "test-token-123"
	exp := 24 * time.Hour
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectations
	mock.ExpectSAdd(userSetKey, token).SetVal(1)
	mock.ExpectExpire(userSetKey, exp).SetVal(true)

	// Call the actual function with mock client
	err := addSessionToUserSetWithClient(db, userID, token, exp)
	if err != nil {
		t.Fatalf("addSessionToUserSetWithClient failed: %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestAddSessionToUserSet_NilClient(t *testing.T) {
	userID := uint(123)
	token := "test-token-123"
	exp := 24 * time.Hour

	// Call the function with nil client
	err := addSessionToUserSetWithClient(nil, userID, token, exp)
	if err != nil {
		t.Fatalf("expected nil error for nil client, got %v", err)
	}
}

func TestAddSessionToUserSet_SAddError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	token := "test-token-123"
	exp := 24 * time.Hour
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectation for SAdd to fail
	expectedErr := errors.New("redis connection error")
	mock.ExpectSAdd(userSetKey, token).SetErr(expectedErr)

	// Call the actual function
	err := addSessionToUserSetWithClient(db, userID, token, exp)
	if err == nil {
		t.Fatal("expected error from addSessionToUserSetWithClient, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestAddSessionToUserSet_ExpireError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	token := "test-token-123"
	exp := 24 * time.Hour
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectations: SAdd succeeds but Expire fails
	mock.ExpectSAdd(userSetKey, token).SetVal(1)
	expectedErr := errors.New("expire failed")
	mock.ExpectExpire(userSetKey, exp).SetErr(expectedErr)

	// Call the actual function
	err := addSessionToUserSetWithClient(db, userID, token, exp)
	if err == nil {
		t.Fatal("expected error from addSessionToUserSetWithClient, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestRemoveSessionTokenFromUserSet_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	token := "test-token-123"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectation
	mock.ExpectSRem(userSetKey, token).SetVal(1)

	// Call the actual function
	err := removeSessionTokenFromUserSetWithClient(db, userID, token)
	if err != nil {
		t.Fatalf("removeSessionTokenFromUserSetWithClient failed: %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestRemoveSessionTokenFromUserSet_NilClient(t *testing.T) {
	userID := uint(123)
	token := "test-token-123"

	// Call the function with nil client
	err := removeSessionTokenFromUserSetWithClient(nil, userID, token)
	if err != nil {
		t.Fatalf("expected nil error for nil client, got %v", err)
	}
}

func TestRemoveSessionTokenFromUserSet_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	token := "test-token-123"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectation for SRem to fail
	expectedErr := errors.New("redis connection error")
	mock.ExpectSRem(userSetKey, token).SetErr(expectedErr)

	// Call the actual function
	err := removeSessionTokenFromUserSetWithClient(db, userID, token)
	if err == nil {
		t.Fatal("expected error from removeSessionTokenFromUserSetWithClient, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestInvalidateUserSessions_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)
	tokens := []string{"token1", "token2", "token3"}

	// Set expectations
	mock.ExpectSMembers(userSetKey).SetVal(tokens)
	for _, tok := range tokens {
		sessionKey := fmt.Sprintf("session:%s", tok)
		mock.ExpectDel(sessionKey).SetVal(1)
	}
	mock.ExpectDel(userSetKey).SetVal(1)

	// Call the actual function
	err := invalidateUserSessionsWithClient(db, userID)
	if err != nil {
		t.Fatalf("invalidateUserSessionsWithClient failed: %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestInvalidateUserSessions_NilClient(t *testing.T) {
	userID := uint(123)

	// Call the function with nil client
	err := invalidateUserSessionsWithClient(nil, userID)
	if err != nil {
		t.Fatalf("expected nil error for nil client, got %v", err)
	}
}

func TestInvalidateUserSessions_EmptySet(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectations for empty set
	mock.ExpectSMembers(userSetKey).SetVal([]string{})
	mock.ExpectDel(userSetKey).SetVal(1)

	// Call the actual function
	err := invalidateUserSessionsWithClient(db, userID)
	if err != nil {
		t.Fatalf("invalidateUserSessionsWithClient failed: %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestInvalidateUserSessions_SMembersError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectation for SMembers to fail with a non-Nil error
	expectedErr := errors.New("redis connection error")
	mock.ExpectSMembers(userSetKey).SetErr(expectedErr)

	// Call the actual function
	err := invalidateUserSessionsWithClient(db, userID)
	if err == nil {
		t.Fatal("expected error from invalidateUserSessionsWithClient, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestInvalidateUserSessions_SMembersNil(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Set expectation for SMembers to return redis.Nil (key doesn't exist)
	mock.ExpectSMembers(userSetKey).RedisNil()
	// Even with redis.Nil, we should still try to delete the key
	mock.ExpectDel(userSetKey).SetVal(0)

	// Call the actual function - redis.Nil should be handled gracefully
	err := invalidateUserSessionsWithClient(db, userID)
	if err != nil {
		t.Fatalf("expected no error when SMembers returns redis.Nil, got %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestInvalidateUserSessions_DelError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	defer db.Close()

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)
	tokens := []string{"token1"}

	// Set expectations: SMembers succeeds, Del fails
	mock.ExpectSMembers(userSetKey).SetVal(tokens)
	sessionKey := fmt.Sprintf("session:%s", tokens[0])
	mock.ExpectDel(sessionKey).SetVal(1)

	expectedErr := errors.New("del failed")
	mock.ExpectDel(userSetKey).SetErr(expectedErr)

	// Call the actual function
	err := invalidateUserSessionsWithClient(db, userID)
	if err == nil {
		t.Fatal("expected error from invalidateUserSessionsWithClient, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
