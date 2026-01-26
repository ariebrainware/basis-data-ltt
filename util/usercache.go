package util

import (
	"container/list"
	"os"
	"strconv"
	"sync"

	"gorm.io/gorm"
)

// LRU cache for userID -> email
type userEntry struct {
	userID uint
	email  string
}

type userLRU struct {
	mu       sync.Mutex
	ll       *list.List
	cache    map[uint]*list.Element
	capacity int
}

var userCache *userLRU

// InitUserEmailCache initializes the LRU cache with given capacity.
// If capacity <= 0, a default of 1000 is used.
func InitUserEmailCache(capacity int) {
	if capacity <= 0 {
		capacity = 1000
	}
	userCache = &userLRU{
		ll:       list.New(),
		cache:    make(map[uint]*list.Element),
		capacity: capacity,
	}
}

// UserEmailCacheGet returns email and true if present in cache.
func UserEmailCacheGet(userID uint) (string, bool) {
	if userCache == nil {
		return "", false
	}
	userCache.mu.Lock()
	defer userCache.mu.Unlock()
	if ele, ok := userCache.cache[userID]; ok {
		userCache.ll.MoveToFront(ele)
		if e, ok := ele.Value.(userEntry); ok {
			return e.email, true
		}
	}
	return "", false
}

// UserEmailCacheSet sets the email for a userID in the cache.
func UserEmailCacheSet(userID uint, email string) {
	if userCache == nil {
		return
	}
	userCache.mu.Lock()
	defer userCache.mu.Unlock()
	if ele, ok := userCache.cache[userID]; ok {
		userCache.ll.MoveToFront(ele)
		ele.Value = userEntry{userID: userID, email: email}
		return
	}
	ele := userCache.ll.PushFront(userEntry{userID: userID, email: email})
	userCache.cache[userID] = ele
	if userCache.ll.Len() > userCache.capacity {
		// evict least recently used
		tail := userCache.ll.Back()
		if tail != nil {
			if e, ok := tail.Value.(userEntry); ok {
				delete(userCache.cache, e.userID)
			}
			userCache.ll.Remove(tail)
		}
	}
}

// GetUserEmail returns the email for userID using cache, falling back to DB.
// If found in DB, caches the result.
func GetUserEmail(db *gorm.DB, userID uint) string {
	if userID == 0 {
		return ""
	}
	if email, ok := UserEmailCacheGet(userID); ok {
		return email
	}
	if db == nil {
		return ""
	}
	var u struct{ Email string }
	if err := db.Table("users").Select("email").Where("id = ?", userID).Take(&u).Error; err == nil {
		if u.Email != "" {
			UserEmailCacheSet(userID, u.Email)
		}
		return u.Email
	}
	return ""
}

// InitUserEmailCacheFromEnv initializes the cache using the env var USER_EMAIL_CACHE_SIZE
func InitUserEmailCacheFromEnv() {
	sizeStr := os.Getenv("USER_EMAIL_CACHE_SIZE")
	if sizeStr == "" {
		InitUserEmailCache(0)
		return
	}
	if n, err := strconv.Atoi(sizeStr); err == nil {
		InitUserEmailCache(n)
		return
	}
	InitUserEmailCache(0)
}
