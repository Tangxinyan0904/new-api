package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

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
