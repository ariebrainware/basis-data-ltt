package util

import (
	"context"
	"fmt"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/redis/go-redis/v9"
)

// AddSessionToUserSet adds the session token to the per-user Redis set.
// The set has no TTL and persists until explicitly cleaned up via
// RemoveSessionTokenFromUserSet or InvalidateUserSessions.
func AddSessionToUserSet(userID uint, token string, exp time.Duration) error {
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
	if err := rdb.SRem(ctx, userSetKey, token).Err(); err != nil {
		return err
	}
	// Check if the set is empty and delete it if so
	count, err := rdb.SCard(ctx, userSetKey).Result()
	if err != nil {
		return err
	}
	if count == 0 {
		return rdb.Del(ctx, userSetKey).Err()
	}
	return nil
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
