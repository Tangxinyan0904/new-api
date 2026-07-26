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
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const distillationDetectionWindow = time.Minute

type distillationRuntimeStore interface {
	RequestWithCount(ctx context.Context, key string, maximum int, window time.Duration) (bool, int, error)
	Delete(ctx context.Context, keys ...string) error
}

type distillationPenaltyStore interface {
	Get(userID int, now int64) (*model.DistillationPenalty, error)
	Advance(userID int, now int64, penaltySeconds int64, observationSeconds int64) (*model.DistillationPenalty, error)
}

type distillationRateLimitDependencies struct {
	runtime   distillationRuntimeStore
	penalties distillationPenaltyStore
	now       func() time.Time
}

func CheckDistillationRateLimit(c *gin.Context, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
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
	if isStream || !settings.Enabled {
		return nil
	}
	if err := setting.ValidateDistillationRateLimitSettings(settings); err != nil {
		return newDistillationStorageError(err)
	}

	if penalty != nil && penalty.Phase(now.Unix()) == model.DistillationPenaltyPhaseTemporary {
		return enforceDistillationTemporaryLimit(ctx, userID, settings.RPM, dependencies.runtime)
	}

	allowed, count, err := dependencies.runtime.RequestWithCount(
		ctx,
		distillationDetectionKey(userID),
		settings.Threshold,
		distillationDetectionWindow,
	)
	if err != nil {
		return newDistillationStorageError(err)
	}
	if count < settings.Threshold {
		return nil
	}

	transitioned, err := dependencies.penalties.Advance(
		userID,
		now.Unix(),
		int64(settings.PenaltyMinutes)*60,
		int64(settings.ObservationMinutes)*60,
	)
	if err != nil {
		_ = dependencies.runtime.Delete(ctx, distillationDetectionKey(userID))
		return newDistillationStorageError(err)
	}
	if transitioned == nil {
		return newDistillationStorageError(errors.New("distillation transition returned no state"))
	}
	if err := dependencies.runtime.Delete(ctx, distillationDetectionKey(userID)); err != nil {
		return newDistillationStorageError(err)
	}
	if allowed {
		return nil
	}

	switch transitioned.Phase(now.Unix()) {
	case model.DistillationPenaltyPhaseTemporary:
		return enforceDistillationTemporaryLimit(ctx, userID, settings.RPM, dependencies.runtime)
	case model.DistillationPenaltyPhasePermanent:
		return newDistillationBannedError()
	default:
		return newDistillationStorageError(fmt.Errorf("unexpected post-threshold penalty phase %s", transitioned.Phase(now.Unix())))
	}
}

func enforceDistillationTemporaryLimit(
	ctx context.Context,
	userID int,
	rpm int,
	runtimeStore distillationRuntimeStore,
) *types.NewAPIError {
	allowed, _, err := runtimeStore.RequestWithCount(
		ctx,
		distillationTemporaryKey(userID),
		rpm,
		distillationDetectionWindow,
	)
	if err != nil {
		return newDistillationStorageError(err)
	}
	if allowed {
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
		errors.New("model API access is permanently suspended after repeated distillation detection; contact an administrator to restore access"),
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
