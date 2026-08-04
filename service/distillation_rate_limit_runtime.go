package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const (
	distillationMemoryPartitionCount = 32
	distillationCounterTTL           = 2 * time.Minute
)

var distillationFixedMinuteScript = redis.NewScript(`
local maximum = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local count = redis.call('INCR', KEYS[1])

if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ttl)
end

local allowed = 0
if count <= maximum then
  allowed = 1
end

local crossed = 0
if count == maximum then
  crossed = 1
end

return {allowed, count, crossed}
`)

type distillationCounterResult struct {
	Allowed bool
	Count   int
	Crossed bool
}

type distillationMemoryCounter struct {
	minute        int64
	count         int
	touchedMinute int64
}

type distillationMemoryPartition struct {
	mu       sync.Mutex
	counters map[string]distillationMemoryCounter
}

type memoryDistillationRuntimeStore struct {
	partitions [distillationMemoryPartitionCount]distillationMemoryPartition
}

func newMemoryDistillationRuntimeStore() *memoryDistillationRuntimeStore {
	store := &memoryDistillationRuntimeStore{}
	for index := range store.partitions {
		store.partitions[index].counters = make(map[string]distillationMemoryCounter)
	}
	return store
}

func (store *memoryDistillationRuntimeStore) Take(
	_ context.Context,
	key string,
	maximum int,
	now time.Time,
) (distillationCounterResult, error) {
	if maximum <= 0 {
		return distillationCounterResult{Allowed: true}, nil
	}
	minute := now.Unix() / 60
	partition := store.partition(key)
	partition.mu.Lock()
	defer partition.mu.Unlock()

	counter := partition.counters[key]
	if counter.minute != minute {
		counter.minute = minute
		counter.count = 0
	}
	counter.count++
	counter.touchedMinute = minute
	partition.counters[key] = counter
	return distillationCounterResult{
		Allowed: counter.count <= maximum,
		Count:   counter.count,
		Crossed: counter.count == maximum,
	}, nil
}

func (store *memoryDistillationRuntimeStore) Delete(
	_ context.Context,
	_ time.Time,
	keys ...string,
) error {
	for _, key := range keys {
		partition := store.partition(key)
		partition.mu.Lock()
		delete(partition.counters, key)
		partition.mu.Unlock()
	}
	return nil
}

func (store *memoryDistillationRuntimeStore) partition(key string) *distillationMemoryPartition {
	hash := uint32(2166136261)
	for index := range len(key) {
		hash ^= uint32(key[index])
		hash *= 16777619
	}
	return &store.partitions[hash%distillationMemoryPartitionCount]
}

func (store *memoryDistillationRuntimeStore) cleanupExpired(now time.Time) {
	oldestActiveMinute := now.Unix()/60 - 2
	for index := range store.partitions {
		partition := &store.partitions[index]
		partition.mu.Lock()
		for key, counter := range partition.counters {
			if counter.touchedMinute < oldestActiveMinute {
				delete(partition.counters, key)
			}
		}
		partition.mu.Unlock()
	}
}

var (
	distillationMemoryRuntime     = newMemoryDistillationRuntimeStore()
	distillationMemoryCleanupOnce sync.Once
)

func startDistillationMemoryCleanup() {
	distillationMemoryCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for now := range ticker.C {
				distillationMemoryRuntime.cleanupExpired(now)
			}
		}()
	})
}

type redisDistillationRuntimeStore struct {
	client *redis.Client
}

func (store redisDistillationRuntimeStore) Take(
	ctx context.Context,
	key string,
	maximum int,
	now time.Time,
) (distillationCounterResult, error) {
	if maximum <= 0 {
		return distillationCounterResult{Allowed: true}, nil
	}
	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result, err := distillationFixedMinuteScript.Run(
		commandCtx,
		store.client,
		[]string{distillationMinuteKey(key, now)},
		maximum,
		distillationCounterTTL.Milliseconds(),
	).Int64Slice()
	if err != nil {
		return distillationCounterResult{}, err
	}
	if len(result) != 3 {
		return distillationCounterResult{}, fmt.Errorf("unexpected distillation rate limit result length %d", len(result))
	}
	return distillationCounterResult{
		Allowed: result[0] == 1,
		Count:   int(result[1]),
		Crossed: result[2] == 1,
	}, nil
}

func (store redisDistillationRuntimeStore) Delete(
	ctx context.Context,
	now time.Time,
	keys ...string,
) error {
	if len(keys) == 0 {
		return nil
	}
	minuteKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		minuteKeys = append(minuteKeys, distillationMinuteKey(key, now))
	}
	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return store.client.Unlink(commandCtx, minuteKeys...).Err()
}

func distillationMinuteKey(key string, now time.Time) string {
	return fmt.Sprintf("%s:%d", key, now.Unix()/60)
}

func currentDistillationRuntimeStore() (distillationRuntimeStore, error) {
	if !common.RedisEnabled {
		startDistillationMemoryCleanup()
		return distillationMemoryRuntime, nil
	}
	if common.RDB == nil {
		return nil, fmt.Errorf("Redis is enabled but unavailable")
	}
	return redisDistillationRuntimeStore{client: common.RDB}, nil
}
