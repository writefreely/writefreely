package writefreely

import (
	"sync"
	"time"
)

const (
	postsCacheTime = 4 * time.Second
)

type (
	postsCacheItem struct {
		Expire time.Time
		Posts  *[]PublicPost
		ready  chan struct{}
	}

	AuthCache struct {
		Alias, Pass, Token string
		BadPasses          map[string]bool

		expire time.Time
	}
)

var (
	userPostsCache = struct {
		sync.RWMutex
		users map[int64]postsCacheItem
	}{
		users: map[int64]postsCacheItem{},
	}
)

func CachePosts(userID int64, p *[]PublicPost) {
	close(userPostsCache.users[userID].ready)
	userPostsCache.Lock()
	userPostsCache.users[userID] = postsCacheItem{
		Expire: time.Now().Add(postsCacheTime),
		Posts:  p,
	}
	userPostsCache.Unlock()
}

func GetPostsCache(userID int64) *[]PublicPost {
	userPostsCache.RLock()
	pci, ok := userPostsCache.users[userID]
	userPostsCache.RUnlock()
	if !ok {
		return nil
	}

	if pci.Expire.Before(time.Now()) {
		// Cache is expired
		return nil
	}
	return pci.Posts
}
