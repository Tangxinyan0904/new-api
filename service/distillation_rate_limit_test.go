package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDistillationRuntimeStore struct {
	counts      map[string]int
	requestKeys []string
	deletedKeys []string
}

func (store *fakeDistillationRuntimeStore) RequestWithCount(
	_ context.Context,
	key string,
	maximum int,
	_ time.Duration,
) (bool, int, error) {
	if store.counts == nil {
		store.counts = make(map[string]int)
	}
	store.requestKeys = append(store.requestKeys, key)
	count := store.counts[key]
	if count >= maximum {
		return false, count, nil
	}
	count++
	store.counts[key] = count
	return true, count, nil
}

func (store *fakeDistillationRuntimeStore) Delete(_ context.Context, keys ...string) error {
	for _, key := range keys {
		delete(store.counts, key)
		store.deletedKeys = append(store.deletedKeys, key)
	}
	return nil
}

type fakeDistillationPenaltyStore struct {
	current        *model.DistillationPenalty
	advanceResults []*model.DistillationPenalty
	getCalls       int
	advanceCalls   int
}

func (store *fakeDistillationPenaltyStore) Get(userID int, _ int64) (*model.DistillationPenalty, error) {
	store.getCalls++
	if store.current == nil {
		return nil, nil
	}
	copy := *store.current
	copy.UserId = userID
	return &copy, nil
}

func (store *fakeDistillationPenaltyStore) Advance(
	userID int,
	_ int64,
	_ int64,
	_ int64,
) (*model.DistillationPenalty, error) {
	store.advanceCalls++
	if len(store.advanceResults) == 0 {
		return nil, nil
	}
	result := *store.advanceResults[0]
	store.advanceResults = store.advanceResults[1:]
	result.UserId = userID
	store.current = &result
	return &result, nil
}

func enabledDistillationSettings() setting.DistillationRateLimitSettings {
	return setting.DistillationRateLimitSettings{
		Enabled:            true,
		Threshold:          2,
		RPM:                1,
		PenaltyMinutes:     10,
		ObservationMinutes: 60,
	}
}

func TestDistillationRateLimitStreamsSkipDetectionButHonorPermanentBan(t *testing.T) {
	runtimeStore := &fakeDistillationRuntimeStore{}
	penaltyStore := &fakeDistillationPenaltyStore{
		current: &model.DistillationPenalty{
			TemporaryLimitedUntil: 1600,
			ObservationUntil:      5200,
		},
	}
	dependencies := distillationRateLimitDependencies{
		runtime:   runtimeStore,
		penalties: penaltyStore,
		now:       func() time.Time { return time.Unix(1200, 0) },
	}

	err := checkDistillationRateLimit(context.Background(), 7, true, enabledDistillationSettings(), dependencies)
	assert.Nil(t, err)
	assert.Empty(t, runtimeStore.requestKeys)

	penaltyStore.current.PermanentlyBannedAt = 1100
	err = checkDistillationRateLimit(context.Background(), 7, true, enabledDistillationSettings(), dependencies)
	require.NotNil(t, err)
	assert.Equal(t, types.ErrorCodeDistillationBanned, err.GetErrorCode())
	assert.Equal(t, 403, err.StatusCode)
}

func TestDistillationThresholdRequestPassesThenTemporaryRPMIsEnforced(t *testing.T) {
	runtimeStore := &fakeDistillationRuntimeStore{}
	penaltyStore := &fakeDistillationPenaltyStore{
		advanceResults: []*model.DistillationPenalty{
			{
				FirstTriggeredAt:      1000,
				TemporaryLimitedUntil: 1600,
				ObservationUntil:      5200,
			},
		},
	}
	dependencies := distillationRateLimitDependencies{
		runtime:   runtimeStore,
		penalties: penaltyStore,
		now:       func() time.Time { return time.Unix(1000, 0) },
	}
	settings := enabledDistillationSettings()

	assert.Nil(t, checkDistillationRateLimit(context.Background(), 8, false, settings, dependencies))
	assert.Nil(t, checkDistillationRateLimit(context.Background(), 8, false, settings, dependencies))
	assert.Equal(t, 1, penaltyStore.advanceCalls)

	assert.Nil(t, checkDistillationRateLimit(context.Background(), 8, false, settings, dependencies))
	err := checkDistillationRateLimit(context.Background(), 8, false, settings, dependencies)
	require.NotNil(t, err)
	assert.Equal(t, types.ErrorCodeDistillationRateLimited, err.GetErrorCode())
	assert.Equal(t, 429, err.StatusCode)

	assert.Equal(t, []string{
		distillationDetectionKey(8),
		distillationDetectionKey(8),
		distillationTemporaryKey(8),
		distillationTemporaryKey(8),
	}, runtimeStore.requestKeys)
}

func TestRequestAfterThresholdUsesPenaltyInsteadOfPassingAsAnotherThresholdRequest(t *testing.T) {
	runtimeStore := &fakeDistillationRuntimeStore{
		counts: map[string]int{
			distillationDetectionKey(18): 2,
			distillationTemporaryKey(18): 1,
		},
	}
	penaltyStore := &fakeDistillationPenaltyStore{
		advanceResults: []*model.DistillationPenalty{
			{
				FirstTriggeredAt:      1000,
				TemporaryLimitedUntil: 1600,
				ObservationUntil:      5200,
			},
		},
	}
	dependencies := distillationRateLimitDependencies{
		runtime:   runtimeStore,
		penalties: penaltyStore,
		now:       func() time.Time { return time.Unix(1000, 0) },
	}

	err := checkDistillationRateLimit(
		context.Background(),
		18,
		false,
		enabledDistillationSettings(),
		dependencies,
	)

	require.NotNil(t, err)
	assert.Equal(t, types.ErrorCodeDistillationRateLimited, err.GetErrorCode())
	assert.Equal(t, []string{
		distillationDetectionKey(18),
		distillationTemporaryKey(18),
	}, runtimeStore.requestKeys)
}

func TestDisabledDistillationBypassesTemporaryLimitButKeepsPermanentBan(t *testing.T) {
	runtimeStore := &fakeDistillationRuntimeStore{}
	penaltyStore := &fakeDistillationPenaltyStore{
		current: &model.DistillationPenalty{
			TemporaryLimitedUntil: 1600,
			ObservationUntil:      5200,
		},
	}
	dependencies := distillationRateLimitDependencies{
		runtime:   runtimeStore,
		penalties: penaltyStore,
		now:       func() time.Time { return time.Unix(1200, 0) },
	}
	settings := enabledDistillationSettings()
	settings.Enabled = false

	assert.Nil(t, checkDistillationRateLimit(context.Background(), 9, false, settings, dependencies))
	assert.Empty(t, runtimeStore.requestKeys)

	penaltyStore.current.PermanentlyBannedAt = 1100
	err := checkDistillationRateLimit(context.Background(), 9, false, settings, dependencies)
	require.NotNil(t, err)
	assert.Equal(t, types.ErrorCodeDistillationBanned, err.GetErrorCode())
}

func TestSecondObservationTriggerBansStartingWithNextRequest(t *testing.T) {
	runtimeStore := &fakeDistillationRuntimeStore{}
	penaltyStore := &fakeDistillationPenaltyStore{
		current: &model.DistillationPenalty{
			FirstTriggeredAt:      1000,
			TemporaryLimitedUntil: 1600,
			ObservationUntil:      5200,
		},
		advanceResults: []*model.DistillationPenalty{
			{
				FirstTriggeredAt:      1000,
				TemporaryLimitedUntil: 1600,
				ObservationUntil:      5200,
				PermanentlyBannedAt:   1700,
			},
		},
	}
	dependencies := distillationRateLimitDependencies{
		runtime:   runtimeStore,
		penalties: penaltyStore,
		now:       func() time.Time { return time.Unix(1700, 0) },
	}
	settings := enabledDistillationSettings()

	assert.Nil(t, checkDistillationRateLimit(context.Background(), 10, false, settings, dependencies))
	assert.Nil(t, checkDistillationRateLimit(context.Background(), 10, false, settings, dependencies))
	err := checkDistillationRateLimit(context.Background(), 10, false, settings, dependencies)
	require.NotNil(t, err)
	assert.Equal(t, types.ErrorCodeDistillationBanned, err.GetErrorCode())
}
