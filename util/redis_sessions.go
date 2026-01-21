package util

import (
	"context"
	"fmt"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/redis/go-redis/v9"
)

// AddSessionToUserSet adds the session token to the per-user Redis set and
// sets the TTL to `exp` so the set expires when the last session expires.
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
	return rdb.Expire(ctx, userSetKey, exp).Err()
}

// RemoveSessionTokenFromUserSet removes a single session token from the per-user set.
func RemoveSessionTokenFromUserSet(userID uint, token string) error {
	rdb := config.GetRedisClient()
	if rdb == nil {
		return nil
	}
	ctx := context.Background()
	userSetKey := fmt.Sprintf("user_sessions:%d", userID)
	return rdb.SRem(ctx, userSetKey, token).Err()
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
	// redis.Nil indicates the key doesn't exist (no active sessions), which is a valid scenario
	if err != nil && err != redis.Nil {
		return err
	}
	for _, tok := range members {
		_ = rdb.Del(ctx, fmt.Sprintf("session:%s", tok)).Err()
	}
	return rdb.Del(ctx, userSetKey).Err()
}
