package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string][]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	l.mutex.Lock()
	if l.store != nil {
		l.mutex.Unlock()
		return
	}
	l.store = make(map[string][]int64)
	l.expirationDuration = expirationDuration
	l.mutex.Unlock()

	if expirationDuration > 0 {
		go l.clearExpiredItems()
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		now := time.Now().Unix()
		for key, queue := range l.store {
			if len(queue) == 0 || now-queue[len(queue)-1] > int64(l.expirationDuration.Seconds()) {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
}

// Check reports whether a request may proceed without recording it.
// The duration unit is seconds; a zero maximum means unlimited.
func (l *InMemoryRateLimiter) Check(key string, maxRequestNum int, duration int64) bool {
	if maxRequestNum <= 0 {
		return true
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.ensureStoreLocked()
	queue := l.pruneExpiredLocked(key, time.Now().Unix(), duration)
	return len(queue) < maxRequestNum
}

// Record adds one event after pruning expired entries from the same window.
// The duration unit is seconds.
func (l *InMemoryRateLimiter) Record(key string, duration int64) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.ensureStoreLocked()
	now := time.Now().Unix()
	queue := l.pruneExpiredLocked(key, now, duration)
	l.store[key] = append(queue, now)
}

// RequestWithCount atomically checks and records one event. It returns the
// current in-window count after the operation, or the unchanged count when
// the request is rejected. The duration unit is seconds.
func (l *InMemoryRateLimiter) RequestWithCount(key string, maxRequestNum int, duration int64) (bool, int) {
	if maxRequestNum <= 0 {
		return true, 0
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.ensureStoreLocked()
	now := time.Now().Unix()
	queue := l.pruneExpiredLocked(key, now, duration)
	if len(queue) >= maxRequestNum {
		return false, len(queue)
	}
	queue = append(queue, now)
	l.store[key] = queue
	return true, len(queue)
}

// Request is the compatibility wrapper used by existing middleware.
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	allowed, _ := l.RequestWithCount(key, maxRequestNum, duration)
	return allowed
}

func (l *InMemoryRateLimiter) Delete(key string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store == nil {
		return
	}
	delete(l.store, key)
}

func (l *InMemoryRateLimiter) ensureStoreLocked() {
	if l.store == nil {
		l.store = make(map[string][]int64)
	}
}

func (l *InMemoryRateLimiter) pruneExpiredLocked(key string, now int64, duration int64) []int64 {
	queue := l.store[key]
	firstActive := 0
	for firstActive < len(queue) && now-queue[firstActive] >= duration {
		firstActive++
	}
	if firstActive > 0 {
		queue = append([]int64(nil), queue[firstActive:]...)
		if len(queue) == 0 {
			delete(l.store, key)
		} else {
			l.store[key] = queue
		}
	}
	return queue
}
