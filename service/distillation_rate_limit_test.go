package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDistillationRuntimeStore struct {
	results     []distillationCounterResult
	requestKeys []string
	requestNow  []time.Time
	deletedKeys []string
	deletedNow  []time.Time
}

func (store *fakeDistillationRuntimeStore) Take(
	_ context.Context,
	key string,
	_ int,
	now time.Time,
) (distillationCounterResult, error) {
	store.requestKeys = append(store.requestKeys, key)
	store.requestNow = append(store.requestNow, now)
	if len(store.results) == 0 {
		return distillationCounterResult{}, errors.New("unexpected distillation counter request")
	}
	result := store.results[0]
	store.results = store.results[1:]
	return result, nil
}

func (store *fakeDistillationRuntimeStore) Delete(_ context.Context, now time.Time, keys ...string) error {
	for _, key := range keys {
		store.deletedKeys = append(store.deletedKeys, key)
		store.deletedNow = append(store.deletedNow, now)
	}
	return nil
}

type fakeDistillationPenaltyStore struct {
	current        *model.DistillationPenalty
	advanceResults []*model.DistillationPenalty
	advanceErrors  []error
	triggers       []model.DistillationTrigger
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
	trigger model.DistillationTrigger,
) (*model.DistillationPenalty, error) {
	store.advanceCalls++
	store.triggers = append(store.triggers, trigger)
	if len(store.advanceErrors) > 0 {
		err := store.advanceErrors[0]
		store.advanceErrors = store.advanceErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(store.advanceResults) == 0 {
		return nil, nil
	}
	result := *store.advanceResults[0]
	store.advanceResults = store.advanceResults[1:]
	result.UserId = trigger.UserID
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
	runtimeStore := &fakeDistillationRuntimeStore{
		results: []distillationCounterResult{
			{Allowed: true, Count: 1},
			{Allowed: true, Count: 2, Crossed: true},
			{Allowed: true, Count: 1, Crossed: true},
			{Allowed: false, Count: 2},
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
	settings := enabledDistillationSettings()

	assert.Nil(t, checkDistillationRateLimit(context.Background(), 8, false, settings, dependencies))
	assert.Nil(t, checkDistillationRateLimit(context.Background(), 8, false, settings, dependencies))
	assert.Equal(t, 1, penaltyStore.advanceCalls)
	require.Len(t, penaltyStore.triggers, 1)
	assert.Equal(t, model.DistillationTrigger{
		UserID:             8,
		TriggeredAt:        1000,
		PenaltySeconds:     600,
		ObservationSeconds: 3600,
		RequestCount:       2,
		DetectionThreshold: 2,
		PenaltyRPM:         1,
	}, penaltyStore.triggers[0])

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
	assert.Equal(t, []time.Time{
		time.Unix(1000, 0),
		time.Unix(1000, 0),
		time.Unix(1000, 0),
		time.Unix(1000, 0),
	}, runtimeStore.requestNow)
}

func TestRequestAfterThresholdUsesPenaltyInsteadOfPassingAsAnotherThresholdRequest(t *testing.T) {
	runtimeStore := &fakeDistillationRuntimeStore{
		results: []distillationCounterResult{
			{Allowed: false, Count: 3},
			{Allowed: false, Count: 2},
		},
	}
	penaltyStore := &fakeDistillationPenaltyStore{}
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
	assert.Zero(t, penaltyStore.advanceCalls)
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
	runtimeStore := &fakeDistillationRuntimeStore{
		results: []distillationCounterResult{
			{Allowed: true, Count: 1},
			{Allowed: true, Count: 2, Crossed: true},
		},
	}
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
	assert.Equal(t, 1, penaltyStore.advanceCalls)
}

func TestDistillationTransitionFailureDeletesDetectionBucketAndCanRetry(t *testing.T) {
	runtimeStore := &fakeDistillationRuntimeStore{
		results: []distillationCounterResult{
			{Allowed: true, Count: 2, Crossed: true},
			{Allowed: true, Count: 1},
			{Allowed: true, Count: 2, Crossed: true},
		},
	}
	penaltyStore := &fakeDistillationPenaltyStore{
		advanceErrors: []error{errors.New("database unavailable"), nil},
		advanceResults: []*model.DistillationPenalty{
			{
				FirstTriggeredAt:      1000,
				TemporaryLimitedUntil: 1600,
				ObservationUntil:      5200,
			},
		},
	}
	now := time.Unix(1000, 0)
	dependencies := distillationRateLimitDependencies{
		runtime:   runtimeStore,
		penalties: penaltyStore,
		now:       func() time.Time { return now },
	}
	settings := enabledDistillationSettings()

	err := checkDistillationRateLimit(context.Background(), 19, false, settings, dependencies)
	require.NotNil(t, err)
	assert.Equal(t, types.ErrorCodeQueryDataError, err.GetErrorCode())
	assert.Equal(t, []string{distillationDetectionKey(19)}, runtimeStore.deletedKeys)
	assert.Equal(t, []time.Time{now}, runtimeStore.deletedNow)

	assert.Nil(t, checkDistillationRateLimit(context.Background(), 19, false, settings, dependencies))
	assert.Nil(t, checkDistillationRateLimit(context.Background(), 19, false, settings, dependencies))
	assert.Equal(t, 2, penaltyStore.advanceCalls)
}
