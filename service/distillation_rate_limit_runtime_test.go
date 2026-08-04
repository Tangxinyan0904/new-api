package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedMinuteDistillationCounter interface {
	Take(context.Context, string, int, time.Time) (distillationCounterResult, error)
}

func TestMemoryDistillationFixedMinuteCounter(t *testing.T) {
	store := newMemoryDistillationRuntimeStore()
	assertDistillationFixedMinuteCounterContract(t, store)
}

func TestRedisDistillationFixedMinuteCounter(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := redisDistillationRuntimeStore{client: client}

	assertDistillationFixedMinuteCounterContract(t, store)

	keys, err := client.Keys(context.Background(), "*").Result()
	require.NoError(t, err)
	require.Len(t, keys, 2)
	for _, key := range keys {
		keyType, typeErr := client.Type(context.Background(), key).Result()
		require.NoError(t, typeErr)
		assert.Equal(t, "string", keyType)
		assert.NotContains(t, key, "sequence")
	}
}

func TestRedisDistillationFixedMinuteCrossingIsAtomic(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := redisDistillationRuntimeStore{client: client}

	const maximum = 8
	start := make(chan struct{})
	results := make(chan distillationCounterResult, maximum)
	errors := make(chan error, maximum)
	var workers sync.WaitGroup
	workers.Add(maximum)
	for range maximum {
		go func() {
			defer workers.Done()
			<-start
			result, err := store.Take(context.Background(), "detection:atomic", maximum, time.Unix(125, 0))
			results <- result
			errors <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}
	allowedCount := 0
	crossedCount := 0
	for result := range results {
		if result.Allowed {
			allowedCount++
		}
		if result.Crossed {
			crossedCount++
		}
	}
	assert.Equal(t, maximum, allowedCount)
	assert.Equal(t, 1, crossedCount)
}

func assertDistillationFixedMinuteCounterContract(t *testing.T, store fixedMinuteDistillationCounter) {
	t.Helper()
	ctx := context.Background()

	first, err := store.Take(ctx, "detection:7", 2, time.Unix(125, 0))
	require.NoError(t, err)
	assert.Equal(t, distillationCounterResult{Allowed: true, Count: 1}, first)

	crossing, err := store.Take(ctx, "detection:7", 2, time.Unix(179, 0))
	require.NoError(t, err)
	assert.Equal(t, distillationCounterResult{Allowed: true, Count: 2, Crossed: true}, crossing)

	over, err := store.Take(ctx, "detection:7", 2, time.Unix(179, 0))
	require.NoError(t, err)
	assert.Equal(t, distillationCounterResult{Allowed: false, Count: 3}, over)

	reset, err := store.Take(ctx, "detection:7", 2, time.Unix(180, 0))
	require.NoError(t, err)
	assert.Equal(t, distillationCounterResult{Allowed: true, Count: 1}, reset)
}
