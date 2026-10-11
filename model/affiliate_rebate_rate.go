package model

import (
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// AffiliateRebateRateChange records when the recharge rebate switch or rate
// changed. A top-up earns the rebate rate that was in effect when it
// completed, so later setting changes never rewrite rebates already earned.
type AffiliateRebateRateChange struct {
	Id              int   `json:"id"`
	EffectiveAt     int64 `json:"effective_at" gorm:"index"`
	Enabled         bool  `json:"enabled"`
	RateBasisPoints int   `json:"rate_basis_points"`
	ChangedBy       int   `json:"changed_by"`
}

type affiliateRebateRateTimeline []AffiliateRebateRateChange

func loadAffiliateRebateRateTimeline(tx *gorm.DB) (affiliateRebateRateTimeline, error) {
	changes := make([]AffiliateRebateRateChange, 0)
	if err := tx.Order("effective_at asc, id asc").Find(&changes).Error; err != nil {
		return nil, err
	}
	return changes, nil
}

// basisPointsAt returns the rebate rate that applied at the given time, or 0
// when the rebate was switched off. Before the first recorded change every
// top-up keeps the legacy 5% rate.
func (timeline affiliateRebateRateTimeline) basisPointsAt(at int64) int {
	index := sort.Search(len(timeline), func(i int) bool {
		return timeline[i].EffectiveAt > at
	})
	if index == 0 {
		return operation_setting.LegacyAffiliateRebateBasisPoints
	}
	change := timeline[index-1]
	if !change.Enabled {
		return 0
	}
	return change.RateBasisPoints
}

func (timeline affiliateRebateRateTimeline) current() (bool, int) {
	if len(timeline) == 0 {
		return true, operation_setting.LegacyAffiliateRebateBasisPoints
	}
	latest := timeline[len(timeline)-1]
	return latest.Enabled, latest.RateBasisPoints
}

// topUpRebateTime is the moment that decides which rebate rate a top-up earns.
func topUpRebateTime(topUp TopUp) int64 {
	if topUp.CompleteTime > 0 {
		return topUp.CompleteTime
	}
	return topUp.CreateTime
}

func affiliateRebateQuota(creditedQuota int, basisPoints int) (int, error) {
	if basisPoints <= 0 || creditedQuota <= 0 {
		return 0, nil
	}
	return affiliateQuotaFromDecimal(
		decimal.NewFromInt(int64(creditedQuota)).
			Mul(decimal.NewFromInt(int64(basisPoints))).
			Div(decimal.NewFromInt(10000)),
	)
}

// affiliateLocalDayStart returns the start of the deployment's local day.
func affiliateLocalDayStart(now int64) int64 {
	year, month, day := time.Unix(now, 0).In(time.Local).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local).Unix()
}

// recordAffiliateRebateRateChange appends a timeline entry when the switch or
// rate differs from the currently effective one.
func recordAffiliateRebateRateChange(tx *gorm.DB, enabled bool, basisPoints int, changedBy int, now int64) error {
	if basisPoints < 0 {
		return errors.New("invalid rebate rate")
	}
	timeline, err := loadAffiliateRebateRateTimeline(tx)
	if err != nil {
		return err
	}
	currentEnabled, currentBasisPoints := timeline.current()
	if currentEnabled == enabled && currentBasisPoints == basisPoints {
		return nil
	}
	return tx.Create(&AffiliateRebateRateChange{
		EffectiveAt:     now,
		Enabled:         enabled,
		RateBasisPoints: basisPoints,
		ChangedBy:       changedBy,
	}).Error
}

// AffiliateOptionsUpdate is the full invitation rebate settings form.
type AffiliateOptionsUpdate struct {
	Setting         operation_setting.AffiliateSetting
	QuotaForInviter int
	QuotaForInvitee int
}

// UpdateAffiliateOptions persists the invitation rebate settings and, in the
// same transaction, records a rate-history entry when the rebate switch or
// rate changed. Later top-ups use the new rate; earlier top-ups keep theirs.
func UpdateAffiliateOptions(update AffiliateOptionsUpdate, changedBy int) error {
	if err := operation_setting.ValidateAffiliateSetting(update.Setting); err != nil {
		return err
	}
	if update.QuotaForInviter < 0 || update.QuotaForInvitee < 0 {
		return errors.New("invitation rewards cannot be negative")
	}
	basisPoints, err := operation_setting.AffiliateRebateBasisPoints(update.Setting.RebateRatePercent)
	if err != nil {
		return err
	}
	prefix := operation_setting.AffiliateSettingOptionPrefix
	values := map[string]string{
		prefix + "rebate_enabled":      strconv.FormatBool(update.Setting.RebateEnabled),
		prefix + "rebate_rate_percent": strconv.FormatFloat(update.Setting.RebateRatePercent, 'f', -1, 64),
		prefix + "min_transfer_quota":  strconv.Itoa(update.Setting.MinTransferQuota),
		prefix + "daily_request_limit": strconv.Itoa(update.Setting.DailyRequestLimit),
		prefix + "require_approval":    strconv.FormatBool(update.Setting.RequireApproval),
		prefix + "forfeit_on_reject":   strconv.FormatBool(update.Setting.ForfeitOnReject),
		"QuotaForInviter":              strconv.Itoa(update.QuotaForInviter),
		"QuotaForInvitee":              strconv.Itoa(update.QuotaForInvitee),
	}
	now := common.GetTimestamp()
	return updateOptionsBulkWithTx(values, func(tx *gorm.DB) error {
		return recordAffiliateRebateRateChange(tx, update.Setting.RebateEnabled, basisPoints, changedBy, now)
	})
}
