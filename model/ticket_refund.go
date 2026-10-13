package model

import "github.com/QuantumNous/new-api/common"

// TicketRefundTopUp is one completed online top-up.
type TicketRefundTopUp struct {
	PaymentMethod string  `json:"payment_method"`
	Amount        int64   `json:"amount"`
	Money         float64 `json:"money"`
	CompleteTime  int64   `json:"complete_time"`
}

// TicketRefundInfo is what staff need to judge a refund request: what the
// user paid, what they redeemed, what is left and how fast they spend it.
type TicketRefundInfo struct {
	// Quota is the user's remaining quota.
	Quota int `json:"quota"`
	// TopUpCount, TopUpAmount and TopUpMoney cover successful online top-ups:
	// how many, the amount credited and the money paid.
	TopUpCount  int64   `json:"top_up_count"`
	TopUpAmount int64   `json:"top_up_amount"`
	TopUpMoney  float64 `json:"top_up_money"`
	// LatestTopUp is the most recent successful online top-up, if any.
	LatestTopUp *TicketRefundTopUp `json:"latest_top_up"`
	// RedemptionCount and RedemptionQuota cover redeemed codes.
	RedemptionCount int64 `json:"redemption_count"`
	RedemptionQuota int64 `json:"redemption_quota"`
	// TodayQuota and WeekQuota are the quota used (consumed minus refunded)
	// since todayStart and weekStart.
	TodayQuota int64 `json:"today_quota"`
	WeekQuota  int64 `json:"week_quota"`
}

// GetTicketRefundInfo collects the refund facts of a user. weekStart must not
// be after todayStart.
func GetTicketRefundInfo(userId int, todayStart, weekStart int64) (*TicketRefundInfo, error) {
	var user User
	if err := DB.Select("id", "quota").Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}
	info := &TicketRefundInfo{Quota: user.Quota}

	var topUps struct {
		Count  int64
		Amount int64
		Money  float64
	}
	if err := DB.Model(&TopUp{}).
		Select("COUNT(*) AS count, COALESCE(SUM(amount), 0) AS amount, COALESCE(SUM(money), 0) AS money").
		Where("user_id = ? AND status = ?", userId, common.TopUpStatusSuccess).
		Scan(&topUps).Error; err != nil {
		return nil, err
	}
	info.TopUpCount, info.TopUpAmount, info.TopUpMoney = topUps.Count, topUps.Amount, topUps.Money
	if topUps.Count > 0 {
		var latest TopUp
		if err := DB.Where("user_id = ? AND status = ?", userId, common.TopUpStatusSuccess).
			Order("complete_time DESC, id DESC").First(&latest).Error; err != nil {
			return nil, err
		}
		info.LatestTopUp = &TicketRefundTopUp{
			PaymentMethod: latest.PaymentMethod,
			Amount:        latest.Amount,
			Money:         latest.Money,
			CompleteTime:  latest.CompleteTime,
		}
	}

	// Redeemed codes still count after an administrator deletes them.
	var redemptions struct {
		Count int64
		Quota int64
	}
	if err := DB.Unscoped().Model(&Redemption{}).
		Select("COUNT(*) AS count, COALESCE(SUM(quota), 0) AS quota").
		Where("used_user_id = ? AND status = ?", userId, common.RedemptionCodeStatusUsed).
		Scan(&redemptions).Error; err != nil {
		return nil, err
	}
	info.RedemptionCount, info.RedemptionQuota = redemptions.Count, redemptions.Quota

	var usage struct {
		Today int64
		Week  int64
	}
	if err := LOG_DB.Model(&Log{}).
		Select(
			"COALESCE(SUM(CASE WHEN created_at >= ? THEN (CASE WHEN type = ? THEN quota ELSE -quota END) ELSE 0 END), 0) AS today, "+
				"COALESCE(SUM(CASE WHEN type = ? THEN quota ELSE -quota END), 0) AS week",
			todayStart, LogTypeConsume, LogTypeConsume,
		).
		Where("user_id = ? AND created_at >= ? AND type IN ?", userId, weekStart, []int{LogTypeConsume, LogTypeRefund}).
		Scan(&usage).Error; err != nil {
		return nil, err
	}
	info.TodayQuota, info.WeekQuota = usage.Today, usage.Week
	return info, nil
}
