package cache

import (
	"container/list"
	"sync"
	"time"
)

// localStore 进程内降级存储：容量受限的 LRU + 按条目 TTL。
//
// 用途（doc89 §3.3 降级矩阵）：Redis 不可用时，验证码/频控/失败计数等
// 仍能以「单实例语义」运行。多实例部署下这里的结果不可跨实例，因此图形
// 验证码在降级期采用「放行」策略（doc91 §2.4），不把 local 结果当强校验依据。
type localStore struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List // 头部最新，尾部最旧
}

type localEntry struct {
	key      string
	value    string
	expireAt time.Time // 零值表示永不过期
}

func newLocalStore(capacity int) *localStore {
	if capacity <= 0 {
		capacity = 4096
	}
	return &localStore{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

func (s *localStore) get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[key]
	if !ok {
		return "", false
	}
	entry := el.Value.(*localEntry)
	if !entry.expireAt.IsZero() && time.Now().After(entry.expireAt) {
		s.removeElement(el)
		return "", false
	}
	s.order.MoveToFront(el)
	return entry.value, true
}

func (s *localStore) set(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}
	if el, ok := s.items[key]; ok {
		entry := el.Value.(*localEntry)
		entry.value = value
		entry.expireAt = expireAt
		s.order.MoveToFront(el)
		return
	}
	el := s.order.PushFront(&localEntry{key: key, value: value, expireAt: expireAt})
	s.items[key] = el
	for s.order.Len() > s.capacity {
		s.removeElement(s.order.Back())
	}
}

// setNX 仅当 key 不存在（或已过期）时写入，返回是否写入成功。
func (s *localStore) setNX(key, value string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.items[key]; ok {
		entry := el.Value.(*localEntry)
		if entry.expireAt.IsZero() || time.Now().Before(entry.expireAt) {
			s.order.MoveToFront(el)
			return false
		}
		s.removeElement(el)
	}
	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}
	el := s.order.PushFront(&localEntry{key: key, value: value, expireAt: expireAt})
	s.items[key] = el
	for s.order.Len() > s.capacity {
		s.removeElement(s.order.Back())
	}
	return true
}

func (s *localStore) del(keys ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		if el, ok := s.items[key]; ok {
			s.removeElement(el)
		}
	}
}

func (s *localStore) incr(key string, ttl time.Duration) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if el, ok := s.items[key]; ok {
		entry := el.Value.(*localEntry)
		if entry.expireAt.IsZero() || now.Before(entry.expireAt) {
			v := parseInt64(entry.value) + 1
			entry.value = formatInt64(v)
			s.order.MoveToFront(el)
			return v
		}
		s.removeElement(el)
	}
	var expireAt time.Time
	if ttl > 0 {
		expireAt = now.Add(ttl)
	}
	el := s.order.PushFront(&localEntry{key: key, value: "1", expireAt: expireAt})
	s.items[key] = el
	for s.order.Len() > s.capacity {
		s.removeElement(s.order.Back())
	}
	return 1
}

func (s *localStore) ttl(key string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[key]
	if !ok {
		return -2 * time.Second
	}
	entry := el.Value.(*localEntry)
	if entry.expireAt.IsZero() {
		return -1 * time.Second
	}
	remaining := time.Until(entry.expireAt)
	if remaining <= 0 {
		s.removeElement(el)
		return -2 * time.Second
	}
	return remaining
}

func (s *localStore) removeElement(el *list.Element) {
	if el == nil {
		return
	}
	entry := el.Value.(*localEntry)
	delete(s.items, entry.key)
	s.order.Remove(el)
}
