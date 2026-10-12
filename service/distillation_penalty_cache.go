package service

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/go-redis/redis/v8"
	"github.com/samber/hot"
)

const (
	distillationPenaltyCacheNamespace = "new-api:rate-limit:distillation:penalty:v1"
	distillationPenaltyMissingValue   = "-"
)

// distillationPenaltyCacheEntry pairs the cache with the Redis client it was
// built for, so the cache follows common.RDB if that client is replaced.
type distillationPenaltyCacheEntry struct {
	client *redis.Client
	cache  *cachex.HybridCache[string]
}

var distillationPenaltyCacheState atomic.Pointer[distillationPenaltyCacheEntry]

type cachedDistillationPenaltyStore struct{}

func (cachedDistillationPenaltyStore) Get(userID int, now int64) (*model.DistillationPenalty, error) {
	if common.RedisEnabled && common.RDB == nil {
		return nil, fmt.Errorf("Redis is enabled but unavailable")
	}
	cache := getDistillationPenaltyCache()
	key := strconv.Itoa(userID)
	value, found, err := cache.Get(key)
	if err != nil {
		return nil, err
	}
	if found {
		if value == distillationPenaltyMissingValue {
			return nil, nil
		}
		var penalty model.DistillationPenalty
		if err := common.UnmarshalJsonStr(value, &penalty); err != nil {
			return nil, err
		}
		if penalty.Phase(now) != model.DistillationPenaltyPhaseClean {
			return &penalty, nil
		}
		if _, err := cache.DeleteMany([]string{key}); err != nil {
			return nil, err
		}
	}

	penalty, err := model.GetDistillationPenalty(userID, now)
	if err != nil {
		return nil, err
	}
	if penalty == nil {
		if err := cache.SetWithTTL(key, distillationPenaltyMissingValue, 30*time.Second); err != nil {
			return nil, err
		}
		return nil, nil
	}
	encoded, err := common.Marshal(penalty)
	if err != nil {
		return nil, err
	}
	if err := cache.SetWithTTL(key, string(encoded), distillationPenaltyTTL(penalty, now)); err != nil {
		return nil, err
	}
	return penalty, nil
}

func (cachedDistillationPenaltyStore) Advance(trigger model.DistillationTrigger) (*model.DistillationPenalty, error) {
	penalty, err := model.AdvanceDistillationPenalty(trigger)
	if err != nil {
		return nil, err
	}
	if err := invalidateDistillationPenaltyCache(trigger.UserID); err != nil {
		return nil, err
	}
	return penalty, nil
}

func (cachedDistillationPenaltyStore) Clear(userID int) error {
	if err := model.ClearDistillationPenalty(userID); err != nil {
		return err
	}
	return invalidateDistillationPenaltyCache(userID)
}

func getDistillationPenaltyCache() *cachex.HybridCache[string] {
	client := common.RDB
	if entry := distillationPenaltyCacheState.Load(); entry != nil && entry.client == client {
		return entry.cache
	}
	entry := &distillationPenaltyCacheEntry{
		client: client,
		cache: cachex.NewHybridCache[string](cachex.HybridCacheConfig[string]{
			Namespace: cachex.Namespace(distillationPenaltyCacheNamespace),
			Redis:     client,
			RedisEnabled: func() bool {
				return common.RedisEnabled && client != nil
			},
			RedisCodec: cachex.StringCodec{},
			Memory: func() *hot.HotCache[string, string] {
				return hot.NewHotCache[string, string](hot.LRU, 10000).
					WithTTL(time.Minute).
					WithJanitor().
					Build()
			},
		}),
	}
	distillationPenaltyCacheState.Store(entry)
	return entry.cache
}

func invalidateDistillationPenaltyCache(userID int) error {
	_, err := getDistillationPenaltyCache().DeleteMany([]string{strconv.Itoa(userID)})
	return err
}

func distillationPenaltyTTL(penalty *model.DistillationPenalty, now int64) time.Duration {
	ttl := time.Minute
	var transitionAt int64
	switch penalty.Phase(now) {
	case model.DistillationPenaltyPhaseTemporary:
		transitionAt = penalty.TemporaryLimitedUntil
	case model.DistillationPenaltyPhaseObservation:
		transitionAt = penalty.ObservationUntil
	}
	if transitionAt <= now {
		return ttl
	}
	untilTransition := time.Duration(transitionAt-now) * time.Second
	if untilTransition < ttl {
		return untilTransition
	}
	return ttl
}
