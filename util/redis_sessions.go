package util

import (
	"context"
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/redis/go-redis/v9"
)

// AddSessionToUserSet adds the session token to the per-user Redis set.
// The set has no TTL and persists until explicitly cleaned up via
// RemoveSessionTokenFromUserSet or InvalidateUserSessions.
func AddSessionToUserSet(userID uint, token string) error {
	rdb := config.GetRedisClient()
	if rdb == nil {
		return nil
	}
	ctx := context.Background()
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)
	if err := rdb.SAdd(ctx, userSetKey, token).Err(); err != nil {
		return err
	}
	// Use PERSIST to ensure the set has no TTL and relies on explicit cleanup
	return rdb.Persist(ctx, userSetKey).Err()
}

// RemoveSessionTokenFromUserSet removes a single session token from the per-user set.
// If the set becomes empty after removal, it is deleted.
func RemoveSessionTokenFromUserSet(userID uint, token string) error {
	rdb := config.GetRedisClient()
	if rdb == nil {
		return nil
	}
	ctx := context.Background()
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)
	// Use a Lua script to atomically remove the token and delete the set if empty
	script := `
		local removed = redis.call('SREM', KEYS[1], ARGV[1])
		if removed > 0 then
			local count = redis.call('SCARD', KEYS[1])
			if count == 0 then
				redis.call('DEL', KEYS[1])
			end
		end
		return removed
	`
	return rdb.Eval(ctx, script, []string{userSetKey}, token).Err()
}

// InvalidateUserSessions deletes all session:<token> keys for the given user and
// removes the per-user set. Best-effort: it will return an error if Redis calls
// fail, but callers may choose to ignore it.
func InvalidateUserSessions(userID uint) error {
	rdb := config.GetRedisClient()
	if rdb == nil {
		return nil
	}
	ctx := context.Background()
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)
	members, err := rdb.SMembers(ctx, userSetKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	for _, tok := range members {
		_ = rdb.Del(ctx, fmt.Sprintf("session:%s", tok)).Err()
	}
	return rdb.Del(ctx, userSetKey).Err()
}
