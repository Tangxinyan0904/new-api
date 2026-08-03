package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

var distillationMemoryRuntime common.InMemoryRateLimiter

var distillationSlidingWindowScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local maximum = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
local count = redis.call('ZCARD', KEYS[1])
if count >= maximum then
  redis.call('PEXPIRE', KEYS[1], window)
  redis.call('PEXPIRE', KEYS[2], window)
  return {0, count}
end

local sequence = redis.call('INCR', KEYS[2])
local member = tostring(now) .. '-' .. tostring(sequence)
redis.call('ZADD', KEYS[1], now, member)
redis.call('PEXPIRE', KEYS[1], window)
redis.call('PEXPIRE', KEYS[2], window)
return {1, count + 1}
`)

type memoryDistillationRuntimeStore struct{}

func (memoryDistillationRuntimeStore) RequestWithCount(
	_ context.Context,
	key string,
	maximum int,
	window time.Duration,
) (bool, int, error) {
	distillationMemoryRuntime.Init(2 * time.Minute)
	allowed, count := distillationMemoryRuntime.RequestWithCount(key, maximum, int64(window.Seconds()))
	return allowed, count, nil
}

func (memoryDistillationRuntimeStore) Delete(_ context.Context, keys ...string) error {
	for _, key := range keys {
		distillationMemoryRuntime.Delete(key)
	}
	return nil
}

type redisDistillationRuntimeStore struct {
	client *redis.Client
}

func (store redisDistillationRuntimeStore) RequestWithCount(
	ctx context.Context,
	key string,
	maximum int,
	window time.Duration,
) (bool, int, error) {
	if maximum <= 0 {
		return true, 0, nil
	}
	windowMilliseconds := window.Milliseconds()
	if windowMilliseconds <= 0 {
		windowMilliseconds = 1
	}
	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result, err := distillationSlidingWindowScript.Run(
		commandCtx,
		store.client,
		[]string{key, key + ":sequence"},
		time.Now().UnixMilli(),
		windowMilliseconds,
		maximum,
	).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(result) != 2 {
		return false, 0, fmt.Errorf("unexpected distillation rate limit result length %d", len(result))
	}
	return result[0] == 1, int(result[1]), nil
}

func (store redisDistillationRuntimeStore) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	redisKeys := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		redisKeys = append(redisKeys, key, key+":sequence")
	}
	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return store.client.Unlink(commandCtx, redisKeys...).Err()
}

func currentDistillationRuntimeStore() (distillationRuntimeStore, error) {
	if !common.RedisEnabled {
		return memoryDistillationRuntimeStore{}, nil
	}
	if common.RDB == nil {
		return nil, fmt.Errorf("Redis is enabled but unavailable")
	}
	return redisDistillationRuntimeStore{client: common.RDB}, nil
}
