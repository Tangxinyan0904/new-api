package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type registrationIPAllowlistRequest struct {
	IP string `json:"ip"`
}

func handleSelfServiceRegistrationError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, model.ErrRegistrationIPBlocked):
		common.ApiErrorI18n(c, i18n.MsgRegistrationIPBlocked)
	case errors.Is(err, model.ErrRegistrationIPLimitExceeded):
		common.ApiErrorI18n(c, i18n.MsgRegistrationIPLimitExceeded)
	case errors.Is(err, model.ErrInvalidRegistrationIP):
		common.ApiErrorI18n(c, i18n.MsgRegistrationIPInvalid)
	default:
		return false
	}
	return true
}

func respondToRegistrationIPLimit(
	c *gin.Context,
	result *model.SelfServiceRegistrationResult,
) bool {
	if result == nil || !result.TriggeredBlock {
		return false
	}
	common.ApiErrorI18n(c, i18n.MsgRegistrationIPLimitExceeded)
	return true
}

func ListBlockedRegistrationIPs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListBlockedRegistrationIPs(c.Query("keyword"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func UnblockRegistrationIP(c *gin.Context) {
	result, err := model.UnblockRegistrationIP(c.Param("ip"))
	if err != nil {
		if !handleSelfServiceRegistrationError(c, err) {
			common.ApiError(c, err)
		}
		return
	}
	recordRegistrationIPMutation(c, "registration_ip.unblock", result)
	common.ApiSuccess(c, result)
}

func ListRegistrationIPAllowlist(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListRegistrationIPAllowlist(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AddRegistrationIPAllowlist(c *gin.Context) {
	var request registrationIPAllowlistRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	result, err := model.AddRegistrationIPAllowlist(request.IP)
	if err != nil {
		if !handleSelfServiceRegistrationError(c, err) {
			common.ApiError(c, err)
		}
		return
	}
	recordRegistrationIPMutation(c, "registration_ip.allowlist_add", result)
	common.ApiSuccess(c, result)
}

func RemoveRegistrationIPAllowlist(c *gin.Context) {
	result, err := model.RemoveRegistrationIPAllowlist(c.Param("ip"))
	if err != nil {
		if !handleSelfServiceRegistrationError(c, err) {
			common.ApiError(c, err)
		}
		return
	}
	recordRegistrationIPMutation(c, "registration_ip.allowlist_remove", result)
	common.ApiSuccess(c, result)
}

func recordRegistrationIPMutation(
	c *gin.Context,
	action string,
	result *model.RegistrationIPMutationResult,
) {
	if result.AffectedUserIDs == nil {
		result.AffectedUserIDs = make([]int, 0)
	}
	recordManageAudit(c, action, map[string]interface{}{
		"ip":                     result.CanonicalIP,
		"affected_account_count": result.AffectedAccountCount,
		"affected_user_ids":      result.AffectedUserIDs,
		"allowlisted":            result.Allowlisted,
	})
}
