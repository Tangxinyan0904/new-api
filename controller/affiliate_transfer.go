package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetAffiliateRebateSummary(c *gin.Context) {
	summary, err := model.GetAffiliateRebateSummary(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func CreateAffiliateTransferRequest(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	request, err := model.CreateAffiliateTransferRequest(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Status == model.AffiliateTransferStatusApproved {
		userLogParams := map[string]interface{}{
			"request_id": request.Id,
			"quota":      request.TotalQuota,
			"amount":     logger.LogQuota(request.TotalQuota),
		}
		model.RecordOperationAuditLog(
			request.UserId,
			auditContentEN("affiliate.transfer.auto_approved", userLogParams),
			c.ClientIP(),
			"affiliate.transfer.auto_approved",
			userLogParams,
			nil,
			nil,
		)
	}
	common.ApiSuccess(c, request)
}

type affiliateOptionsUpdateRequest struct {
	RebateEnabled     *bool    `json:"rebate_enabled"`
	RebateRatePercent *float64 `json:"rebate_rate_percent"`
	MinTransferQuota  *int     `json:"min_transfer_quota"`
	DailyRequestLimit *int     `json:"daily_request_limit"`
	RequireApproval   *bool    `json:"require_approval"`
	ForfeitOnReject   *bool    `json:"forfeit_on_reject"`
	QuotaForInviter   *int     `json:"quota_for_inviter"`
	QuotaForInvitee   *int     `json:"quota_for_invitee"`
}

// UpdateAffiliateOptions saves the whole invitation rebate settings form.
// Rebate-rate history is written with the options, so these keys are not
// accepted by the generic option endpoint.
func UpdateAffiliateOptions(c *gin.Context) {
	var request affiliateOptionsUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "invalid invitation rebate settings")
		return
	}
	if request.RebateEnabled == nil || request.RebateRatePercent == nil || request.MinTransferQuota == nil ||
		request.DailyRequestLimit == nil || request.RequireApproval == nil || request.ForfeitOnReject == nil ||
		request.QuotaForInviter == nil || request.QuotaForInvitee == nil {
		common.ApiErrorMsg(c, "incomplete invitation rebate settings")
		return
	}
	if (*request.QuotaForInviter > 0 || *request.QuotaForInvitee > 0) && !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}
	update := model.AffiliateOptionsUpdate{
		Setting: operation_setting.AffiliateSetting{
			RebateEnabled:     *request.RebateEnabled,
			RebateRatePercent: *request.RebateRatePercent,
			MinTransferQuota:  *request.MinTransferQuota,
			DailyRequestLimit: *request.DailyRequestLimit,
			RequireApproval:   *request.RequireApproval,
			ForfeitOnReject:   *request.ForfeitOnReject,
		},
		QuotaForInviter: *request.QuotaForInviter,
		QuotaForInvitee: *request.QuotaForInvitee,
	}
	if err := model.UpdateAffiliateOptions(update, c.GetInt("id")); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "affiliate.settings.update", map[string]interface{}{
		"rebate_enabled":      update.Setting.RebateEnabled,
		"rebate_rate_percent": update.Setting.RebateRatePercent,
		"min_transfer_quota":  update.Setting.MinTransferQuota,
		"daily_request_limit": update.Setting.DailyRequestLimit,
		"require_approval":    update.Setting.RequireApproval,
		"forfeit_on_reject":   update.Setting.ForfeitOnReject,
		"quota_for_inviter":   update.QuotaForInviter,
		"quota_for_invitee":   update.QuotaForInvitee,
	})
	common.ApiSuccess(c, nil)
}

func ListAffiliateTransferRequests(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListAffiliateTransferRequests(c.Query("status"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func ListSelfAffiliateTransferRequests(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListUserAffiliateTransferRequests(c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetAffiliateTransferRequestDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	detail, err := model.GetAffiliateTransferRequestDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

func ApproveAffiliateTransferRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	detail, err := model.GetAffiliateTransferRequestDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ApproveAffiliateTransferRequest(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	recordAffiliateTransferApproval(c, detail)
	common.ApiSuccess(c, nil)
}

type approveAllAffiliateTransferRequestsResult struct {
	ApprovedCount int `json:"approved_count"`
}

func ApproveAllAffiliateTransferRequests(c *gin.Context) {
	details, err := model.ApproveAllPendingAffiliateTransferRequests(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, detail := range details {
		recordAffiliateTransferApproval(c, detail)
	}
	common.ApiSuccess(c, approveAllAffiliateTransferRequestsResult{ApprovedCount: len(details)})
}

func recordAffiliateTransferApproval(c *gin.Context, detail *model.AffiliateTransferRequestDetail) {
	recordManageAudit(c, "affiliate.transfer.approve", affiliateTransferAuditParams(detail, ""))
	userLogParams := map[string]interface{}{
		"request_id": detail.Id,
		"quota":      detail.TotalQuota,
		"amount":     logger.LogQuota(detail.TotalQuota),
	}
	model.RecordOperationAuditLog(
		detail.UserId,
		auditContentEN("affiliate.transfer.approved_for_user", userLogParams),
		c.ClientIP(),
		"affiliate.transfer.approved_for_user",
		userLogParams,
		auditOperatorInfo(c),
		nil,
	)
}

type rejectAffiliateTransferRequestBody struct {
	Reason string `json:"reason"`
}

func RejectAffiliateTransferRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	body := rejectAffiliateTransferRequestBody{}
	_ = c.ShouldBindJSON(&body)
	detail, err := model.GetAffiliateTransferRequestDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.RejectAffiliateTransferRequest(id, c.GetInt("id"), body.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "affiliate.transfer.reject", affiliateTransferAuditParams(detail, body.Reason))
	common.ApiSuccess(c, nil)
}

func affiliateTransferAuditParams(detail *model.AffiliateTransferRequestDetail, reason string) map[string]interface{} {
	params := map[string]interface{}{
		"request_id":            detail.Id,
		"target_user_id":        detail.UserId,
		"target_username":       detail.Username,
		"target_display_name":   detail.DisplayName,
		"invite_reward_quota":   detail.InviteRewardQuota,
		"recharge_rebate_quota": detail.RechargeRebateQuota,
		"total_quota":           detail.TotalQuota,
	}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		params["reason"] = trimmed
	}
	return params
}
