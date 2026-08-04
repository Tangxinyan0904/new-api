package model

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DistillationPenaltyPhase string

const (
	DistillationPenaltyPhaseClean       DistillationPenaltyPhase = "clean"
	DistillationPenaltyPhaseTemporary   DistillationPenaltyPhase = "temporary"
	DistillationPenaltyPhaseObservation DistillationPenaltyPhase = "observation"
	DistillationPenaltyPhasePermanent   DistillationPenaltyPhase = "permanent"
)

type DistillationPenalty struct {
	Id                    int   `json:"id"`
	UserId                int   `json:"user_id" gorm:"not null;uniqueIndex"`
	FirstTriggeredAt      int64 `json:"first_triggered_at" gorm:"not null"`
	TemporaryLimitedUntil int64 `json:"temporary_limited_until" gorm:"not null"`
	ObservationUntil      int64 `json:"observation_until" gorm:"not null;index"`
	PermanentlyBannedAt   int64 `json:"permanently_banned_at" gorm:"not null;index"`
	CreatedAt             int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt             int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type DistillationPenaltyListItem struct {
	UserId                int                      `json:"user_id"`
	Username              string                   `json:"username"`
	Phase                 DistillationPenaltyPhase `json:"phase"`
	FirstTriggeredAt      int64                    `json:"first_triggered_at"`
	TemporaryLimitedUntil int64                    `json:"temporary_limited_until"`
	ObservationUntil      int64                    `json:"observation_until"`
	PermanentlyBannedAt   int64                    `json:"permanently_banned_at"`
	CreatedAt             int64                    `json:"created_at"`
	UpdatedAt             int64                    `json:"updated_at"`
}

type DistillationTrigger struct {
	UserID             int
	TriggeredAt        int64
	PenaltySeconds     int64
	ObservationSeconds int64
	RequestCount       int
	DetectionThreshold int
	PenaltyRPM         int
}

func (penalty *DistillationPenalty) Phase(now int64) DistillationPenaltyPhase {
	if penalty == nil {
		return DistillationPenaltyPhaseClean
	}
	if penalty.PermanentlyBannedAt > 0 {
		return DistillationPenaltyPhasePermanent
	}
	if now < penalty.TemporaryLimitedUntil {
		return DistillationPenaltyPhaseTemporary
	}
	if now < penalty.ObservationUntil {
		return DistillationPenaltyPhaseObservation
	}
	return DistillationPenaltyPhaseClean
}

func GetDistillationPenalty(userID int, now int64) (*DistillationPenalty, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user ID must be positive")
	}

	var penalty DistillationPenalty
	err := DB.Where("user_id = ?", userID).First(&penalty).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if penalty.Phase(now) != DistillationPenaltyPhaseClean {
		return &penalty, nil
	}

	result := DB.Where(
		"user_id = ? AND permanently_banned_at = ? AND observation_until <= ?",
		userID,
		0,
		now,
	).Delete(&DistillationPenalty{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return nil, nil
	}

	err = DB.Where("user_id = ?", userID).First(&penalty).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if penalty.Phase(now) == DistillationPenaltyPhaseClean {
		return nil, nil
	}
	return &penalty, nil
}

func AdvanceDistillationPenalty(trigger DistillationTrigger) (*DistillationPenalty, error) {
	userID := trigger.UserID
	now := trigger.TriggeredAt
	penaltySeconds := trigger.PenaltySeconds
	observationSeconds := trigger.ObservationSeconds
	if userID <= 0 {
		return nil, fmt.Errorf("user ID must be positive")
	}
	if now < 0 || penaltySeconds <= 0 || observationSeconds <= 0 {
		return nil, fmt.Errorf("distillation transition timestamps and durations must be positive")
	}
	if trigger.RequestCount <= 0 || trigger.DetectionThreshold <= 0 || trigger.RequestCount < trigger.DetectionThreshold || trigger.PenaltyRPM <= 0 {
		return nil, fmt.Errorf("distillation trigger counters and limits must be positive")
	}
	if penaltySeconds > math.MaxInt64-now {
		return nil, fmt.Errorf("distillation penalty timestamp overflow")
	}
	temporaryUntil := now + penaltySeconds
	if observationSeconds > math.MaxInt64-temporaryUntil {
		return nil, fmt.Errorf("distillation observation timestamp overflow")
	}
	observationUntil := temporaryUntil + observationSeconds

	var transitioned DistillationPenalty
	err := DB.Transaction(func(tx *gorm.DB) error {
		for range 3 {
			var current DistillationPenalty
			err := lockForUpdate(tx).Where("user_id = ?", userID).First(&current).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				candidate := DistillationPenalty{
					UserId:                userID,
					FirstTriggeredAt:      now,
					TemporaryLimitedUntil: temporaryUntil,
					ObservationUntil:      observationUntil,
				}
				result := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "user_id"}},
					DoNothing: true,
				}).Create(&candidate)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected > 0 {
					if err := createDistillationViolationRecord(
						tx,
						trigger,
						now,
						DistillationViolationActionTemporaryLimit,
						temporaryUntil,
					); err != nil {
						return err
					}
					transitioned = candidate
					return nil
				}
				continue
			}
			if err != nil {
				return err
			}

			switch current.Phase(now) {
			case DistillationPenaltyPhasePermanent, DistillationPenaltyPhaseTemporary:
				transitioned = current
				return nil
			case DistillationPenaltyPhaseObservation:
				result := tx.Model(&current).
					Where("permanently_banned_at = ?", 0).
					Update("permanently_banned_at", now)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					continue
				}
				if err := createDistillationViolationRecord(
					tx,
					trigger,
					current.FirstTriggeredAt,
					DistillationViolationActionPermanentBan,
					0,
				); err != nil {
					return err
				}
				current.PermanentlyBannedAt = now
				transitioned = current
				return nil
			case DistillationPenaltyPhaseClean:
				result := tx.Where(
					"user_id = ? AND permanently_banned_at = ? AND observation_until <= ?",
					userID,
					0,
					now,
				).Delete(&DistillationPenalty{})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					continue
				}
			}
		}
		return fmt.Errorf("distillation penalty state changed concurrently")
	})
	if err != nil {
		return nil, err
	}
	return &transitioned, nil
}

func ClearDistillationPenalty(userID int) error {
	if userID <= 0 {
		return fmt.Errorf("user ID must be positive")
	}
	return DB.Where("user_id = ?", userID).Delete(&DistillationPenalty{}).Error
}

func ListDistillationPenalties(
	keyword string,
	pageInfo *common.PageInfo,
	now int64,
) ([]*DistillationPenaltyListItem, int64, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = common.ItemsPerPage
	}
	if pageInfo.PageSize > 100 {
		pageInfo.PageSize = 100
	}

	query := DB.Table("distillation_penalties").
		Joins("LEFT JOIN users ON users.id = distillation_penalties.user_id").
		Where("distillation_penalties.permanently_banned_at > ? OR distillation_penalties.observation_until > ?", 0, now)

	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		pattern := "%" + keyword + "%"
		if userID, err := strconv.Atoi(keyword); err == nil && userID > 0 {
			query = query.Where("distillation_penalties.user_id = ? OR users.username LIKE ?", userID, pattern)
		} else {
			query = query.Where("users.username LIKE ?", pattern)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*DistillationPenaltyListItem, 0)
	err := query.Select(
		"distillation_penalties.user_id, users.username, " +
			"distillation_penalties.first_triggered_at, distillation_penalties.temporary_limited_until, " +
			"distillation_penalties.observation_until, distillation_penalties.permanently_banned_at, " +
			"distillation_penalties.created_at, distillation_penalties.updated_at",
	).
		Order("distillation_penalties.updated_at DESC, distillation_penalties.id DESC").
		Limit(pageInfo.PageSize).
		Offset((pageInfo.Page - 1) * pageInfo.PageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	for _, item := range items {
		penalty := DistillationPenalty{
			PermanentlyBannedAt:   item.PermanentlyBannedAt,
			TemporaryLimitedUntil: item.TemporaryLimitedUntil,
			ObservationUntil:      item.ObservationUntil,
		}
		item.Phase = penalty.Phase(now)
	}
	return items, total, nil
}
