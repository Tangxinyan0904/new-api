package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type distillationRuntimeStore interface {
	Take(ctx context.Context, key string, maximum int, now time.Time) (distillationCounterResult, error)
	Delete(ctx context.Context, now time.Time, keys ...string) error
}

type distillationPenaltyStore interface {
	Get(userID int, now int64) (*model.DistillationPenalty, error)
	Advance(trigger model.DistillationTrigger) (*model.DistillationPenalty, error)
}

type distillationRateLimitDependencies struct {
	runtime   distillationRuntimeStore
	penalties distillationPenaltyStore
	now       func() time.Time
}

func CheckDistillationRateLimit(c *gin.Context, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if relayInfo.IsStream {
		return nil
	}
	runtimeStore, err := currentDistillationRuntimeStore()
	if err != nil {
		return newDistillationStorageError(err)
	}
	return checkDistillationRateLimit(
		c.Request.Context(),
		relayInfo.UserId,
		relayInfo.IsStream,
		setting.GetDistillationRateLimitSettings(),
		distillationRateLimitDependencies{
			runtime:   runtimeStore,
			penalties: cachedDistillationPenaltyStore{},
			now:       time.Now,
		},
	)
}

func ClearDistillationRateLimitState(ctx context.Context, userID int) error {
	runtimeStore, err := currentDistillationRuntimeStore()
	if err != nil {
		return err
	}
	penalties := cachedDistillationPenaltyStore{}
	if err := penalties.Clear(userID); err != nil {
		return err
	}
	return runtimeStore.Delete(
		ctx,
		time.Now(),
		distillationDetectionKey(userID),
		distillationTemporaryKey(userID),
	)
}

func checkDistillationRateLimit(
	ctx context.Context,
	userID int,
	isStream bool,
	settings setting.DistillationRateLimitSettings,
	dependencies distillationRateLimitDependencies,
) *types.NewAPIError {
	if isStream {
		return nil
	}
	if userID <= 0 || dependencies.runtime == nil || dependencies.penalties == nil {
		return newDistillationStorageError(errors.New("invalid distillation rate limit context"))
	}
	if dependencies.now == nil {
		dependencies.now = time.Now
	}
	now := dependencies.now()
	penalty, err := dependencies.penalties.Get(userID, now.Unix())
	if err != nil {
		return newDistillationStorageError(err)
	}
	if penalty != nil && penalty.Phase(now.Unix()) == model.DistillationPenaltyPhasePermanent {
		return newDistillationBannedError()
	}
	if !settings.Enabled {
		return nil
	}
	if err := setting.ValidateDistillationRateLimitSettings(settings); err != nil {
		return newDistillationStorageError(err)
	}

	if penalty != nil && penalty.Phase(now.Unix()) == model.DistillationPenaltyPhaseTemporary {
		return enforceDistillationTemporaryLimit(ctx, userID, settings.RPM, now, dependencies.runtime)
	}

	counter, err := dependencies.runtime.Take(
		ctx,
		distillationDetectionKey(userID),
		settings.Threshold,
		now,
	)
	if err != nil {
		return newDistillationStorageError(err)
	}
	if counter.Crossed {
		transitioned, transitionErr := dependencies.penalties.Advance(model.DistillationTrigger{
			UserID:             userID,
			TriggeredAt:        now.Unix(),
			PenaltySeconds:     int64(settings.PenaltyMinutes) * 60,
			ObservationSeconds: int64(settings.ObservationMinutes) * 60,
			RequestCount:       counter.Count,
			DetectionThreshold: settings.Threshold,
			PenaltyRPM:         settings.RPM,
		})
		if transitionErr != nil {
			_ = dependencies.runtime.Delete(ctx, now, distillationDetectionKey(userID))
			return newDistillationStorageError(transitionErr)
		}
		if transitioned == nil {
			_ = dependencies.runtime.Delete(ctx, now, distillationDetectionKey(userID))
			return newDistillationStorageError(errors.New("distillation transition returned no state"))
		}
		return nil
	}
	if counter.Allowed {
		return nil
	}
	return enforceDistillationTemporaryLimit(ctx, userID, settings.RPM, now, dependencies.runtime)
}

func enforceDistillationTemporaryLimit(
	ctx context.Context,
	userID int,
	rpm int,
	now time.Time,
	runtimeStore distillationRuntimeStore,
) *types.NewAPIError {
	counter, err := runtimeStore.Take(
		ctx,
		distillationTemporaryKey(userID),
		rpm,
		now,
	)
	if err != nil {
		return newDistillationStorageError(err)
	}
	if counter.Allowed {
		return nil
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("distillation protection allows at most %d non-stream requests per minute during the penalty period", rpm),
		types.ErrorCodeDistillationRateLimited,
		http.StatusTooManyRequests,
		types.ErrOptionWithSkipRetry(),
	)
}

func newDistillationBannedError() *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New("non-stream model API access is permanently suspended after repeated distillation detection; streaming requests remain available; contact an administrator to restore access"),
		types.ErrorCodeDistillationBanned,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)
}

func distillationDetectionKey(userID int) string {
	return "new-api:rate-limit:distillation:detection:" + strconv.Itoa(userID)
}

func distillationTemporaryKey(userID int) string {
	return "new-api:rate-limit:distillation:temporary:" + strconv.Itoa(userID)
}

func newDistillationStorageError(err error) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		fmt.Errorf("failed to evaluate distillation protection: %w", err),
		types.ErrorCodeQueryDataError,
		http.StatusInternalServerError,
		types.ErrOptionWithSkipRetry(),
	)
}
