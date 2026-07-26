package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type RateLimitSettingsRequest struct {
	ModelRequestRateLimitEnabled                        bool   `json:"ModelRequestRateLimitEnabled"`
	ModelRequestRateLimitDurationMinutes                int    `json:"ModelRequestRateLimitDurationMinutes"`
	ModelRequestRateLimitCount                          int    `json:"ModelRequestRateLimitCount"`
	ModelRequestRateLimitSuccessCount                   int    `json:"ModelRequestRateLimitSuccessCount"`
	ModelRequestRateLimitGroup                          string `json:"ModelRequestRateLimitGroup"`
	ModelRequestRateLimitUser                           string `json:"ModelRequestRateLimitUser"`
	ModelRequestRateLimitDistillationEnabled            bool   `json:"ModelRequestRateLimitDistillationEnabled"`
	ModelRequestRateLimitDistillationThreshold          int    `json:"ModelRequestRateLimitDistillationThreshold"`
	ModelRequestRateLimitDistillationRPM                int    `json:"ModelRequestRateLimitDistillationRPM"`
	ModelRequestRateLimitDistillationPenaltyMinutes     int    `json:"ModelRequestRateLimitDistillationPenaltyMinutes"`
	ModelRequestRateLimitDistillationObservationMinutes int    `json:"ModelRequestRateLimitDistillationObservationMinutes"`
}

func UpdateRateLimitSettings(c *gin.Context) {
	var request RateLimitSettingsRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "invalid rate limit settings")
		return
	}

	settings := setting.ModelRequestRateLimitSettings{
		Enabled:         request.ModelRequestRateLimitEnabled,
		DurationMinutes: request.ModelRequestRateLimitDurationMinutes,
		Count:           request.ModelRequestRateLimitCount,
		SuccessCount:    request.ModelRequestRateLimitSuccessCount,
		GroupJSON:       request.ModelRequestRateLimitGroup,
		UserJSON:        request.ModelRequestRateLimitUser,
		Distillation: setting.DistillationRateLimitSettings{
			Enabled:            request.ModelRequestRateLimitDistillationEnabled,
			Threshold:          request.ModelRequestRateLimitDistillationThreshold,
			RPM:                request.ModelRequestRateLimitDistillationRPM,
			PenaltyMinutes:     request.ModelRequestRateLimitDistillationPenaltyMinutes,
			ObservationMinutes: request.ModelRequestRateLimitDistillationObservationMinutes,
		},
	}
	if err := setting.ValidateModelRequestRateLimitSettings(settings); err != nil {
		common.ApiError(c, err)
		return
	}

	values := map[string]string{
		"ModelRequestRateLimitEnabled":                        common.Interface2String(request.ModelRequestRateLimitEnabled),
		"ModelRequestRateLimitDurationMinutes":                common.Interface2String(request.ModelRequestRateLimitDurationMinutes),
		"ModelRequestRateLimitCount":                          common.Interface2String(request.ModelRequestRateLimitCount),
		"ModelRequestRateLimitSuccessCount":                   common.Interface2String(request.ModelRequestRateLimitSuccessCount),
		"ModelRequestRateLimitGroup":                          request.ModelRequestRateLimitGroup,
		"ModelRequestRateLimitUser":                           request.ModelRequestRateLimitUser,
		"ModelRequestRateLimitDistillationEnabled":            common.Interface2String(request.ModelRequestRateLimitDistillationEnabled),
		"ModelRequestRateLimitDistillationThreshold":          common.Interface2String(request.ModelRequestRateLimitDistillationThreshold),
		"ModelRequestRateLimitDistillationRPM":                common.Interface2String(request.ModelRequestRateLimitDistillationRPM),
		"ModelRequestRateLimitDistillationPenaltyMinutes":     common.Interface2String(request.ModelRequestRateLimitDistillationPenaltyMinutes),
		"ModelRequestRateLimitDistillationObservationMinutes": common.Interface2String(request.ModelRequestRateLimitDistillationObservationMinutes),
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}

	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": "rate-limit",
	})
	common.ApiSuccess(c, nil)
}

func ListDistillationPenalties(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListDistillationPenalties(c.Query("keyword"), pageInfo, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func ClearDistillationPenalty(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		common.ApiError(c, errors.New("user ID must be positive"))
		return
	}
	if err := service.ClearDistillationRateLimitState(c.Request.Context(), userID); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "rate_limit.distillation_clear", map[string]interface{}{
		"target_user_id": userID,
	})
	common.ApiSuccess(c, nil)
}
