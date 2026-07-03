package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	common.ApiSuccess(c, request)
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
	recordManageAudit(c, "affiliate.transfer.approve", affiliateTransferAuditParams(detail, ""))
	common.ApiSuccess(c, nil)
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
