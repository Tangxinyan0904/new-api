package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DistillationViolationAction string

const (
	DistillationViolationActionTemporaryLimit DistillationViolationAction = "temporary_limit"
	DistillationViolationActionPermanentBan   DistillationViolationAction = "permanent_ban"
)

type DistillationViolationRecord struct {
	Id                 int                         `json:"id"`
	UserId             int                         `json:"-" gorm:"not null;index;uniqueIndex:idx_distillation_violation_transition,priority:1"`
	CycleStartedAt     int64                       `json:"cycle_started_at" gorm:"not null;uniqueIndex:idx_distillation_violation_transition,priority:2"`
	TriggeredAt        int64                       `json:"triggered_at" gorm:"not null;index"`
	RequestCount       int                         `json:"request_count" gorm:"not null"`
	DetectionThreshold int                         `json:"detection_threshold" gorm:"not null"`
	PenaltyRPM         int                         `json:"penalty_rpm" gorm:"not null"`
	Action             DistillationViolationAction `json:"action" gorm:"type:varchar(32);not null;uniqueIndex:idx_distillation_violation_transition,priority:3"`
	EffectiveUntil     int64                       `json:"effective_until" gorm:"not null"`
	CreatedAt          int64                       `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

func createDistillationViolationRecord(
	tx *gorm.DB,
	trigger DistillationTrigger,
	cycleStartedAt int64,
	action DistillationViolationAction,
	effectiveUntil int64,
) error {
	record := DistillationViolationRecord{
		UserId:             trigger.UserID,
		CycleStartedAt:     cycleStartedAt,
		TriggeredAt:        trigger.TriggeredAt,
		RequestCount:       trigger.RequestCount,
		DetectionThreshold: trigger.DetectionThreshold,
		PenaltyRPM:         trigger.PenaltyRPM,
		Action:             action,
		EffectiveUntil:     effectiveUntil,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "cycle_started_at"},
			{Name: "action"},
		},
		DoNothing: true,
	}).Create(&record).Error
}

func ListUserDistillationViolationRecords(
	userID int,
	pageInfo *common.PageInfo,
) ([]*DistillationViolationRecord, int64, error) {
	if userID <= 0 {
		return nil, 0, fmt.Errorf("user ID must be positive")
	}
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

	query := DB.Model(&DistillationViolationRecord{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*DistillationViolationRecord, 0)
	if err := query.
		Order("triggered_at DESC, id DESC").
		Limit(pageInfo.PageSize).
		Offset((pageInfo.Page - 1) * pageInfo.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
