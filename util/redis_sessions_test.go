package util

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/go-redis/redismock/v9"
)

// removeSessionLuaScript is the Lua script used in RemoveSessionTokenFromUserSet
// to atomically remove a token and delete the set if empty.
const removeSessionLuaScript = `
		local removed = redis.call('SREM', KEYS[1], ARGV[1])
		if removed > 0 then
			local count = redis.call('SCARD', KEYS[1])
			if count == 0 then
				redis.call('DEL', KEYS[1])
			end
		end
		return removed
	`

func TestAddSessionToUserSet(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	token := "test-token-123"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Expect SAdd and Persist commands
	mock.ExpectSAdd(userSetKey, token).SetVal(1)
	mock.ExpectPersist(userSetKey).SetVal(true)

	err := AddSessionToUserSet(userID, token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAddSessionToUserSet_RedisNotAvailable(t *testing.T) {
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(nil)

	err := AddSessionToUserSet(123, "token")
	if err != nil {
		t.Fatalf("expected no error when Redis is not available, got %v", err)
	}
}

func TestRemoveSessionTokenFromUserSet_TokenRemovedCorrectly(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	token := "test-token-123"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Expect Eval to be called with the script, and return 1 (token removed)
	mock.ExpectEval(removeSessionLuaScript, []string{userSetKey}, token).SetVal(int64(1))

	err := RemoveSessionTokenFromUserSet(userID, token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRemoveSessionTokenFromUserSet_EmptySetDeleted(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	token := "last-token"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// The Lua script will remove the token and delete the set
	mock.ExpectEval(removeSessionLuaScript, []string{userSetKey}, token).SetVal(int64(1))

	err := RemoveSessionTokenFromUserSet(userID, token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRemoveSessionTokenFromUserSet_NonEmptySetPersists(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	token := "token-to-remove"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// The Lua script will remove the token but NOT delete the set (count > 0)
	mock.ExpectEval(removeSessionLuaScript, []string{userSetKey}, token).SetVal(int64(1))

	err := RemoveSessionTokenFromUserSet(userID, token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRemoveSessionTokenFromUserSet_TokenNotFound(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	token := "nonexistent-token"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Lua script returns 0 (token not found)
	mock.ExpectEval(removeSessionLuaScript, []string{userSetKey}, token).SetVal(int64(0))

	err := RemoveSessionTokenFromUserSet(userID, token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRemoveSessionTokenFromUserSet_RedisNotAvailable(t *testing.T) {
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(nil)

	err := RemoveSessionTokenFromUserSet(123, "token")
	if err != nil {
		t.Fatalf("expected no error when Redis is not available, got %v", err)
	}
}

func TestRemoveSessionTokenFromUserSet_ConcurrentRemoval(t *testing.T) {
	// This test verifies that concurrent removal operations are handled correctly
	// by using the atomic Lua script
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	// Allow expectations to be matched out of order for concurrent operations
	mock.MatchExpectationsInOrder(false)

	userID := uint(123)
	token1 := "token-1"
	token2 := "token-2"
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Expect two concurrent Eval calls (order may vary due to concurrency)
	mock.ExpectEval(removeSessionLuaScript, []string{userSetKey}, token1).SetVal(int64(1))
	mock.ExpectEval(removeSessionLuaScript, []string{userSetKey}, token2).SetVal(int64(1))

	var wg sync.WaitGroup
	wg.Add(2)

	errCh := make(chan error, 2)

	// Remove token1 concurrently
	go func() {
		defer wg.Done()
		if err := RemoveSessionTokenFromUserSet(userID, token1); err != nil {
			errCh <- err
		}
	}()

	// Remove token2 concurrently
	go func() {
		defer wg.Done()
		if err := RemoveSessionTokenFromUserSet(userID, token2); err != nil {
			errCh <- err
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("expected no error during concurrent removal, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestInvalidateUserSessions(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)
	tokens := []string{"token1", "token2", "token3"}

	// Expect SMembers to return the list of tokens
	mock.ExpectSMembers(userSetKey).SetVal(tokens)

	// Expect Del for each session token
	for _, token := range tokens {
		mock.ExpectDel(fmt.Sprintf("session:%s", token)).SetVal(1)
	}

	// Expect Del for the user set
	mock.ExpectDel(userSetKey).SetVal(1)

	err := InvalidateUserSessions(userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestInvalidateUserSessions_NoTokens(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Expect SMembers to return an empty list
	mock.ExpectSMembers(userSetKey).SetVal([]string{})

	// Expect Del for the user set
	mock.ExpectDel(userSetKey).SetVal(0)

	err := InvalidateUserSessions(userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestInvalidateUserSessions_KeyNotFound(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Expect SMembers to return redis.Nil (key not found)
	mock.ExpectSMembers(userSetKey).RedisNil()

	// Expect Del for the user set (even though it doesn't exist)
	mock.ExpectDel(userSetKey).SetVal(0)

	err := InvalidateUserSessions(userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestInvalidateUserSessions_RedisNotAvailable(t *testing.T) {
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(nil)

	err := InvalidateUserSessions(123)
	if err != nil {
		t.Fatalf("expected no error when Redis is not available, got %v", err)
	}
}

func TestInvalidateUserSessions_RedisError(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	userID := uint(123)
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)

	// Expect SMembers to return an error (not redis.Nil)
	mock.ExpectSMembers(userSetKey).SetErr(context.DeadlineExceeded)

	err := InvalidateUserSessions(userID)
	if err == nil {
		t.Fatalf("expected error when Redis fails, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
