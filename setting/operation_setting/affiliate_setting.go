package operation_setting

import (
	"errors"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

// AffiliateSettingOptionPrefix is the option-key prefix of the invitation
// rebate settings. These keys are written only through the dedicated
// affiliate settings endpoint so that rebate-rate history stays consistent.
const AffiliateSettingOptionPrefix = "affiliate_setting."

const (
	// LegacyAffiliateRebateBasisPoints is the 5% rate that applied to every
	// top-up before rebate settings became configurable.
	LegacyAffiliateRebateBasisPoints = 500
	MaxAffiliateRebateRatePercent    = 100
	MaxAffiliateDailyRequestLimit    = 100
)

type AffiliateSetting struct {
	// RebateEnabled controls whether new top-ups earn a recharge rebate and
	// whether already earned recharge rebate is visible and withdrawable.
	RebateEnabled bool `json:"rebate_enabled"`
	// RebateRatePercent applies to top-ups completed after it is saved.
	RebateRatePercent float64 `json:"rebate_rate_percent"`
	// MinTransferQuota is the smallest transferable amount; 0 means any
	// positive amount can be transferred.
	MinTransferQuota int `json:"min_transfer_quota"`
	// DailyRequestLimit caps transfer requests per user per local day; 0
	// means unlimited.
	DailyRequestLimit int  `json:"daily_request_limit"`
	RequireApproval   bool `json:"require_approval"`
	ForfeitOnReject   bool `json:"forfeit_on_reject"`
}

var affiliateSetting = AffiliateSetting{
	RebateEnabled:     true,
	RebateRatePercent: LegacyAffiliateRebateBasisPoints / 100,
	MinTransferQuota:  int(common.QuotaPerUnit),
	DailyRequestLimit: 1,
	RequireApproval:   true,
	ForfeitOnReject:   true,
}

func init() {
	config.GlobalConfig.Register("affiliate_setting", &affiliateSetting)
}

// GetAffiliateSetting returns a snapshot of the current invitation rebate settings.
func GetAffiliateSetting() AffiliateSetting {
	return affiliateSetting
}

// ValidateAffiliateSetting checks administrator input before it is persisted.
func ValidateAffiliateSetting(setting AffiliateSetting) error {
	if _, err := AffiliateRebateBasisPoints(setting.RebateRatePercent); err != nil {
		return err
	}
	if setting.MinTransferQuota < 0 || setting.MinTransferQuota > common.MaxQuota {
		return errors.New("minimum transfer quota is out of range")
	}
	if setting.DailyRequestLimit < 0 || setting.DailyRequestLimit > MaxAffiliateDailyRequestLimit {
		return errors.New("daily transfer request limit must be between 0 and 100")
	}
	return nil
}

// AffiliateRebateBasisPoints converts a percentage with at most two decimal
// places into basis points (5.25% -> 525).
func AffiliateRebateBasisPoints(ratePercent float64) (int, error) {
	if math.IsNaN(ratePercent) || math.IsInf(ratePercent, 0) || ratePercent < 0 || ratePercent > MaxAffiliateRebateRatePercent {
		return 0, errors.New("rebate rate must be between 0 and 100 percent")
	}
	basisPoints := decimal.NewFromFloat(ratePercent).Mul(decimal.NewFromInt(100))
	if !basisPoints.Equal(basisPoints.Truncate(0)) {
		return 0, errors.New("rebate rate supports at most two decimal places")
	}
	return int(basisPoints.IntPart()), nil
}
