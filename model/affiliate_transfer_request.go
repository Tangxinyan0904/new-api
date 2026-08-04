package model

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	AffiliateTransferStatusPending  = "pending"
	AffiliateTransferStatusApproved = "approved"
	AffiliateTransferStatusRejected = "rejected"
	AffiliateRechargeRebateRate     = 0.05
)

type AffiliateTransferRequest struct {
	Id                       int    `json:"id"`
	UserId                   int    `json:"user_id" gorm:"index"`
	InviteRewardQuota        int    `json:"invite_reward_quota"`
	RechargeRebateQuota      int    `json:"recharge_rebate_quota"`
	TotalQuota               int    `json:"total_quota"`
	Status                   string `json:"status" gorm:"type:varchar(32);index"`
	CreatedAt                int64  `json:"created_at" gorm:"index"`
	ReviewedAt               int64  `json:"reviewed_at"`
	ReviewedBy               int    `json:"reviewed_by" gorm:"index"`
	RejectReason             string `json:"reject_reason" gorm:"type:varchar(255)"`
	RejectedQuotaForfeitedAt int64  `json:"-" gorm:"column:rejected_quota_forfeited_at"`
}

type AffiliateTransferRequestHistoryItem struct {
	Id                  int    `json:"id"`
	InviteRewardQuota   int    `json:"invite_reward_quota"`
	RechargeRebateQuota int    `json:"recharge_rebate_quota"`
	TotalQuota          int    `json:"total_quota"`
	Status              string `json:"status"`
	CreatedAt           int64  `json:"created_at"`
	ReviewedAt          int64  `json:"reviewed_at"`
	RejectReason        string `json:"reject_reason"`
}

type AffiliateInvitedUserSummary struct {
	Id          int    `json:"id"`
	DisplayName string `json:"display_name"`
}

type AffiliateRebateSummary struct {
	InvitedUsers              []AffiliateInvitedUserSummary `json:"invited_users"`
	InvitedCount              int                           `json:"invited_count"`
	TotalInvitedRechargeQuota int                           `json:"total_invited_recharge_quota"`
	InviteRewardQuota         int                           `json:"invite_reward_quota"`
	RechargeRebateQuota       int                           `json:"recharge_rebate_quota"`
	TotalPendingQuota         int                           `json:"total_pending_quota"`
	PendingRequest            *AffiliateTransferRequest     `json:"pending_request,omitempty"`
	SubmittedToday            bool                          `json:"submitted_today"`
}

type AffiliateTransferRequestListItem struct {
	AffiliateTransferRequest
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type AffiliateInvitedUserDetail struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	CreatedAt   int64  `json:"created_at"`
	LastLoginAt int64  `json:"last_login_at"`
	IsDeleted   bool   `json:"is_deleted"`
}

type AffiliateTransferRequestDetail struct {
	AffiliateTransferRequest
	Username                  string                        `json:"username"`
	DisplayName               string                        `json:"display_name"`
	InvitedUsers              []AffiliateInvitedUserDetail  `json:"invited_users"`
	InvitedCount              int                           `json:"invited_count"`
	TotalInvitedRechargeQuota int                           `json:"total_invited_recharge_quota"`
	RechargeRebateRate        float64                       `json:"recharge_rebate_rate"`
	RechargeSources           []AffiliateRechargeSourceItem `json:"recharge_sources"`
}

type AffiliateRechargeSourceItem struct {
	InvitedUserId      int    `json:"invited_user_id"`
	InvitedDisplayName string `json:"invited_display_name"`
	PaymentProvider    string `json:"payment_provider"`
	PaymentMethod      string `json:"payment_method"`
	CreditedQuota      int    `json:"credited_quota"`
	RebateQuota        int    `json:"rebate_quota"`
	CompleteTime       int64  `json:"complete_time"`
}

func maskAffiliateDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "***"
	}
	chars := []rune(name)
	if len(chars) <= 2 {
		return string(chars[0]) + "***"
	}
	return string(chars[0]) + strings.Repeat("*", len(chars)-2) + string(chars[len(chars)-1])
}

func affiliateQuotaFromDecimal(value decimal.Decimal) (int, error) {
	quota, clamp := common.QuotaFromDecimalChecked(value)
	if clamp != nil {
		return 0, clamp
	}
	if quota < 0 {
		return 0, errors.New("affiliate quota cannot be negative")
	}
	return quota, nil
}

func addAffiliateQuota(left int, right int) (int, error) {
	if left < 0 || right < 0 {
		return 0, errors.New("affiliate quota cannot be negative")
	}
	return affiliateQuotaFromDecimal(decimal.NewFromInt(int64(left)).Add(decimal.NewFromInt(int64(right))))
}

func creditedTopUpQuota(topUp TopUp) (int, error) {
	switch topUp.PaymentProvider {
	case PaymentProviderCreem:
		return affiliateQuotaFromDecimal(decimal.NewFromInt(topUp.Amount))
	case PaymentProviderStripe:
		if math.IsNaN(topUp.Money) || math.IsInf(topUp.Money, 0) {
			return 0, errors.New("invalid top-up money")
		}
		return affiliateQuotaFromDecimal(decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	default:
		return affiliateQuotaFromDecimal(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	}
}

func getAffiliateRebateSummaryWithDB(tx *gorm.DB, userId int) (*AffiliateRebateSummary, error) {
	var user User
	if err := tx.First(&user, "id = ?", userId).Error; err != nil {
		return nil, err
	}

	var invitedUsers []User
	if err := tx.Select("id", "username", "display_name").Where("inviter_id = ?", userId).Order("id desc").Find(&invitedUsers).Error; err != nil {
		return nil, err
	}

	invitedIds := make([]int, 0, len(invitedUsers))
	summaries := make([]AffiliateInvitedUserSummary, 0, len(invitedUsers))
	for _, invited := range invitedUsers {
		invitedIds = append(invitedIds, invited.Id)
		name := invited.DisplayName
		if name == "" {
			name = invited.Username
		}
		summaries = append(summaries, AffiliateInvitedUserSummary{Id: invited.Id, DisplayName: maskAffiliateDisplayName(name)})
	}

	totalRechargeQuota := 0
	if len(invitedIds) > 0 {
		var topUps []TopUp
		if err := tx.Where("user_id IN ? AND status = ?", invitedIds, common.TopUpStatusSuccess).Find(&topUps).Error; err != nil {
			return nil, err
		}
		for _, topUp := range topUps {
			creditedQuota, err := creditedTopUpQuota(topUp)
			if err != nil {
				return nil, err
			}
			totalRechargeQuota, err = addAffiliateQuota(totalRechargeQuota, creditedQuota)
			if err != nil {
				return nil, err
			}
		}
	}

	grossRechargeRebateQuota, err := affiliateQuotaFromDecimal(decimal.NewFromInt(int64(totalRechargeQuota)).Mul(decimal.NewFromFloat(AffiliateRechargeRebateRate)))
	if err != nil {
		return nil, err
	}
	var requestedRechargeRebateQuota int
	if err := tx.Model(&AffiliateTransferRequest{}).
		Where("user_id = ? AND (status <> ? OR rejected_quota_forfeited_at > ?)", userId, AffiliateTransferStatusRejected, 0).
		Select("COALESCE(SUM(recharge_rebate_quota), 0)").
		Scan(&requestedRechargeRebateQuota).Error; err != nil {
		return nil, err
	}
	if requestedRechargeRebateQuota < 0 {
		return nil, errors.New("requested affiliate rebate quota cannot be negative")
	}
	rebateQuota := 0
	if grossRechargeRebateQuota > requestedRechargeRebateQuota {
		rebateQuota = grossRechargeRebateQuota - requestedRechargeRebateQuota
	}
	pendingQuota, err := addAffiliateQuota(user.AffQuota, rebateQuota)
	if err != nil {
		return nil, err
	}

	var pending AffiliateTransferRequest
	pendingRequest := (*AffiliateTransferRequest)(nil)
	if err := tx.Where("user_id = ? AND status = ?", userId, AffiliateTransferStatusPending).Order("id desc").First(&pending).Error; err == nil {
		pendingRequest = &pending
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := common.GetTimestamp()
	startOfDay := now - (now % 86400)
	var todayCount int64
	if err := tx.Model(&AffiliateTransferRequest{}).Where("user_id = ? AND created_at >= ?", userId, startOfDay).Count(&todayCount).Error; err != nil {
		return nil, err
	}

	return &AffiliateRebateSummary{
		InvitedUsers:              summaries,
		InvitedCount:              len(invitedUsers),
		TotalInvitedRechargeQuota: totalRechargeQuota,
		InviteRewardQuota:         user.AffQuota,
		RechargeRebateQuota:       rebateQuota,
		TotalPendingQuota:         pendingQuota,
		PendingRequest:            pendingRequest,
		SubmittedToday:            todayCount > 0,
	}, nil
}

func GetAffiliateRebateSummary(userId int) (*AffiliateRebateSummary, error) {
	return getAffiliateRebateSummaryWithDB(DB, userId)
}

func CreateAffiliateTransferRequest(userId int) (*AffiliateTransferRequest, error) {
	var created AffiliateTransferRequest
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").First(&user, "id = ?", userId).Error; err != nil {
			return err
		}

		var pendingCount int64
		if err := tx.Model(&AffiliateTransferRequest{}).Where("user_id = ? AND status = ?", userId, AffiliateTransferStatusPending).Count(&pendingCount).Error; err != nil {
			return err
		}
		if pendingCount > 0 {
			return errors.New("rebate transfer request is already pending")
		}

		now := common.GetTimestamp()
		startOfDay := now - (now % 86400)
		var todayCount int64
		if err := tx.Model(&AffiliateTransferRequest{}).Where("user_id = ? AND created_at >= ?", userId, startOfDay).Count(&todayCount).Error; err != nil {
			return err
		}
		if todayCount > 0 {
			return errors.New("only one rebate transfer request can be submitted per day")
		}

		summary, err := getAffiliateRebateSummaryWithDB(tx, userId)
		if err != nil {
			return err
		}
		if float64(summary.TotalPendingQuota) < common.QuotaPerUnit {
			return errors.New("insufficient rebate quota to transfer")
		}

		created = AffiliateTransferRequest{
			UserId:              userId,
			InviteRewardQuota:   summary.InviteRewardQuota,
			RechargeRebateQuota: summary.RechargeRebateQuota,
			TotalQuota:          summary.TotalPendingQuota,
			Status:              AffiliateTransferStatusPending,
			CreatedAt:           now,
		}
		return tx.Create(&created).Error
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func ListAffiliateTransferRequests(status string, pageInfo *common.PageInfo) ([]*AffiliateTransferRequestListItem, int64, error) {
	baseQuery := DB.Table("affiliate_transfer_requests").
		Joins("LEFT JOIN users ON users.id = affiliate_transfer_requests.user_id")
	if status != "" {
		baseQuery = baseQuery.Where("affiliate_transfer_requests.status = ?", status)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*AffiliateTransferRequestListItem, 0)
	if err := baseQuery.
		Select("affiliate_transfer_requests.*, users.username, users.display_name").
		Order("affiliate_transfer_requests.id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func ListUserAffiliateTransferRequests(userId int, pageInfo *common.PageInfo) ([]*AffiliateTransferRequestHistoryItem, int64, error) {
	baseQuery := DB.Model(&AffiliateTransferRequest{}).Where("user_id = ?", userId)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*AffiliateTransferRequestHistoryItem, 0)
	if err := baseQuery.
		Select("id, invite_reward_quota, recharge_rebate_quota, total_quota, status, created_at, reviewed_at, reject_reason").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetAffiliateTransferRequestDetail(requestId int) (*AffiliateTransferRequestDetail, error) {
	var item AffiliateTransferRequestListItem
	if err := DB.Table("affiliate_transfer_requests").
		Joins("LEFT JOIN users ON users.id = affiliate_transfer_requests.user_id").
		Select("affiliate_transfer_requests.*, users.username, users.display_name").
		Where("affiliate_transfer_requests.id = ?", requestId).
		Scan(&item).Error; err != nil {
		return nil, err
	}
	if item.Id == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var invitedUsers []User
	if err := DB.Unscoped().
		Select("id", "username", "display_name", "created_at", "last_login_at", "deleted_at").
		Where("inviter_id = ?", item.UserId).
		Order("created_at DESC, id DESC").
		Find(&invitedUsers).Error; err != nil {
		return nil, err
	}

	invitedUserDetails := make([]AffiliateInvitedUserDetail, 0, len(invitedUsers))
	invitedNames := make(map[int]string, len(invitedUsers))
	invitedIds := make([]int, 0, len(invitedUsers))
	for _, invited := range invitedUsers {
		invitedUserDetails = append(invitedUserDetails, AffiliateInvitedUserDetail{
			Id:          invited.Id,
			Username:    invited.Username,
			DisplayName: invited.DisplayName,
			CreatedAt:   invited.CreatedAt,
			LastLoginAt: invited.LastLoginAt,
			IsDeleted:   invited.DeletedAt.Valid,
		})
		if invited.DeletedAt.Valid {
			continue
		}
		invitedIds = append(invitedIds, invited.Id)
		name := invited.DisplayName
		if name == "" {
			name = invited.Username
		}
		invitedNames[invited.Id] = name
	}

	sources := make([]AffiliateRechargeSourceItem, 0)
	totalRechargeQuota := 0
	requestRechargeRebateQuota := item.RechargeRebateQuota
	if requestRechargeRebateQuota > common.MaxQuota {
		return nil, errors.New("affiliate rebate quota exceeds the maximum")
	}
	if len(invitedIds) > 0 && requestRechargeRebateQuota > 0 {
		var previousRechargeRebateQuota int
		if err := DB.Model(&AffiliateTransferRequest{}).
			Where("user_id = ? AND (status <> ? OR rejected_quota_forfeited_at > ?) AND (created_at < ? OR (created_at = ? AND id < ?))", item.UserId, AffiliateTransferStatusRejected, 0, item.CreatedAt, item.CreatedAt, item.Id).
			Select("COALESCE(SUM(recharge_rebate_quota), 0)").
			Scan(&previousRechargeRebateQuota).Error; err != nil {
			return nil, err
		}
		if previousRechargeRebateQuota < 0 {
			return nil, errors.New("requested affiliate rebate quota cannot be negative")
		}

		var topUps []TopUp
		if err := DB.Where("user_id IN ? AND status = ? AND (complete_time = 0 OR complete_time <= ?)", invitedIds, common.TopUpStatusSuccess, item.CreatedAt).Order("complete_time asc, id asc").Find(&topUps).Error; err != nil {
			return nil, err
		}
		for _, topUp := range topUps {
			creditedQuota, err := creditedTopUpQuota(topUp)
			if err != nil {
				return nil, err
			}
			if creditedQuota <= 0 {
				continue
			}
			fullRebateQuota, err := affiliateQuotaFromDecimal(decimal.NewFromInt(int64(creditedQuota)).Mul(decimal.NewFromFloat(AffiliateRechargeRebateRate)))
			if err != nil {
				return nil, err
			}
			if fullRebateQuota <= 0 {
				continue
			}
			if previousRechargeRebateQuota >= fullRebateQuota {
				previousRechargeRebateQuota -= fullRebateQuota
				continue
			}
			sourceRebateQuota := fullRebateQuota - previousRechargeRebateQuota
			previousRechargeRebateQuota = 0
			if sourceRebateQuota > requestRechargeRebateQuota {
				sourceRebateQuota = requestRechargeRebateQuota
			}
			sourceCreditedQuota := creditedQuota
			if sourceRebateQuota < fullRebateQuota {
				sourceCreditedQuota, err = affiliateQuotaFromDecimal(decimal.NewFromInt(int64(sourceRebateQuota)).Div(decimal.NewFromFloat(AffiliateRechargeRebateRate)))
				if err != nil {
					return nil, err
				}
			}
			totalRechargeQuota, err = addAffiliateQuota(totalRechargeQuota, sourceCreditedQuota)
			if err != nil {
				return nil, err
			}
			sources = append(sources, AffiliateRechargeSourceItem{
				InvitedUserId:      topUp.UserId,
				InvitedDisplayName: invitedNames[topUp.UserId],
				PaymentProvider:    topUp.PaymentProvider,
				PaymentMethod:      topUp.PaymentMethod,
				CreditedQuota:      sourceCreditedQuota,
				RebateQuota:        sourceRebateQuota,
				CompleteTime:       topUp.CompleteTime,
			})
			requestRechargeRebateQuota -= sourceRebateQuota
			if requestRechargeRebateQuota <= 0 {
				break
			}
		}
	}

	return &AffiliateTransferRequestDetail{
		AffiliateTransferRequest:  item.AffiliateTransferRequest,
		Username:                  item.Username,
		DisplayName:               item.DisplayName,
		InvitedUsers:              invitedUserDetails,
		InvitedCount:              len(invitedUsers),
		TotalInvitedRechargeQuota: totalRechargeQuota,
		RechargeRebateRate:        AffiliateRechargeRebateRate,
		RechargeSources:           sources,
	}, nil
}

func approveAffiliateTransferRequestWithDB(tx *gorm.DB, request *AffiliateTransferRequest, reviewerId int) error {
	if request.Status != AffiliateTransferStatusPending {
		return errors.New("request is not pending")
	}
	if request.InviteRewardQuota < 0 || request.RechargeRebateQuota < 0 || request.TotalQuota <= 0 || request.TotalQuota > common.MaxQuota {
		return errors.New("invalid affiliate transfer quota")
	}

	res := tx.Model(&AffiliateTransferRequest{}).
		Where("id = ? AND status = ?", request.Id, AffiliateTransferStatusPending).
		Updates(map[string]interface{}{
			"status":        AffiliateTransferStatusApproved,
			"reviewed_at":   common.GetTimestamp(),
			"reviewed_by":   reviewerId,
			"reject_reason": "",
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("request is not pending")
	}

	if request.InviteRewardQuota > 0 {
		res = tx.Model(&User{}).
			Where("id = ? AND aff_quota >= ?", request.UserId, request.InviteRewardQuota).
			Update("aff_quota", gorm.Expr("aff_quota - ?", request.InviteRewardQuota))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("insufficient invitation reward quota")
		}
	}
	res = tx.Model(&User{}).
		Where("id = ? AND quota <= ?", request.UserId, common.MaxQuota-request.TotalQuota).
		Update("quota", gorm.Expr("quota + ?", request.TotalQuota))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("transfer recipient does not exist or quota limit would be exceeded")
	}
	return nil
}

func ApproveAffiliateTransferRequest(requestId int, reviewerId int) error {
	var approvedUserId int
	var approvedQuota int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var request AffiliateTransferRequest
		if err := lockForUpdate(tx).First(&request, "id = ?", requestId).Error; err != nil {
			return err
		}
		if err := approveAffiliateTransferRequestWithDB(tx, &request, reviewerId); err != nil {
			return err
		}
		approvedUserId = request.UserId
		approvedQuota = request.TotalQuota
		return nil
	})
	if err != nil {
		return err
	}
	if err := cacheIncrUserQuota(approvedUserId, int64(approvedQuota)); err != nil {
		common.SysLog("failed to increase user quota cache after affiliate transfer approval: " + err.Error())
	}
	return nil
}

func ApproveAllPendingAffiliateTransferRequests(reviewerId int) ([]*AffiliateTransferRequestDetail, error) {
	approved := make([]*AffiliateTransferRequestDetail, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		requests := make([]AffiliateTransferRequest, 0)
		if err := lockForUpdate(tx).
			Where("status = ?", AffiliateTransferStatusPending).
			Order("id ASC").
			Find(&requests).Error; err != nil {
			return err
		}
		if len(requests) == 0 {
			return nil
		}

		userIds := make([]int, 0, len(requests))
		for _, request := range requests {
			userIds = append(userIds, request.UserId)
		}
		users := make([]User, 0, len(userIds))
		if err := tx.Select("id", "username", "display_name").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return err
		}
		usersById := make(map[int]User, len(users))
		for _, user := range users {
			usersById[user.Id] = user
		}

		for i := range requests {
			request := &requests[i]
			if err := approveAffiliateTransferRequestWithDB(tx, request, reviewerId); err != nil {
				return err
			}
			user := usersById[request.UserId]
			approved = append(approved, &AffiliateTransferRequestDetail{
				AffiliateTransferRequest: *request,
				Username:                 user.Username,
				DisplayName:              user.DisplayName,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, detail := range approved {
		if err := cacheIncrUserQuota(detail.UserId, int64(detail.TotalQuota)); err != nil {
			common.SysLog("failed to increase user quota cache after affiliate transfer approval: " + err.Error())
		}
	}
	return approved, nil
}

func RejectAffiliateTransferRequest(requestId int, reviewerId int, reason string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var request AffiliateTransferRequest
		if err := lockForUpdate(tx).First(&request, "id = ?", requestId).Error; err != nil {
			return err
		}
		if request.Status != AffiliateTransferStatusPending {
			return errors.New("request is not pending")
		}

		now := common.GetTimestamp()
		res := tx.Model(&AffiliateTransferRequest{}).
			Where("id = ? AND status = ?", request.Id, AffiliateTransferStatusPending).
			Updates(map[string]interface{}{
				"status":                      AffiliateTransferStatusRejected,
				"reviewed_at":                 now,
				"reviewed_by":                 reviewerId,
				"reject_reason":               strings.TrimSpace(reason),
				"rejected_quota_forfeited_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("request is not pending")
		}

		if request.InviteRewardQuota > 0 {
			res = tx.Model(&User{}).
				Where("id = ? AND aff_quota >= ?", request.UserId, request.InviteRewardQuota).
				Update("aff_quota", gorm.Expr("aff_quota - ?", request.InviteRewardQuota))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errors.New("insufficient invitation reward quota")
			}
		}
		return nil
	})
}
