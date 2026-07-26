package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rateLimitTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupRateLimitControllerTest(t *testing.T) {
	t.Helper()

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Log{}))

	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	originalEnabled := setting.ModelRequestRateLimitEnabled
	originalDuration := setting.ModelRequestRateLimitDurationMinutes
	originalCount := setting.ModelRequestRateLimitCount
	originalSuccessCount := setting.ModelRequestRateLimitSuccessCount
	originalGroupJSON := setting.ModelRequestRateLimitGroup2JSONString()
	originalUserJSON := setting.ModelRequestRateLimitUser2JSONString()
	originalDistillation := setting.GetDistillationRateLimitSettings()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
		setting.ModelRequestRateLimitEnabled = originalEnabled
		setting.ModelRequestRateLimitDurationMinutes = originalDuration
		setting.ModelRequestRateLimitCount = originalCount
		setting.ModelRequestRateLimitSuccessCount = originalSuccessCount
		require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(originalGroupJSON))
		require.NoError(t, setting.UpdateModelRequestRateLimitUserByJSONString(originalUserJSON))
		require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationEnabled", common.Interface2String(originalDistillation.Enabled)))
		require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationThreshold", common.Interface2String(originalDistillation.Threshold)))
		require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationRPM", common.Interface2String(originalDistillation.RPM)))
		require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationPenaltyMinutes", common.Interface2String(originalDistillation.PenaltyMinutes)))
		require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationObservationMinutes", common.Interface2String(originalDistillation.ObservationMinutes)))
	})
}

func performRateLimitSettingsRequest(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/rate-limit", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)

	UpdateRateLimitSettings(ctx)
	return recorder
}

func TestUpdateRateLimitSettingsPersistsCompleteConfiguration(t *testing.T) {
	setupRateLimitControllerTest(t)

	recorder := performRateLimitSettingsRequest(t, gin.H{
		"ModelRequestRateLimitEnabled":                        true,
		"ModelRequestRateLimitDurationMinutes":                2,
		"ModelRequestRateLimitCount":                          500,
		"ModelRequestRateLimitSuccessCount":                   300,
		"ModelRequestRateLimitGroup":                          `{"vip":[200,100]}`,
		"ModelRequestRateLimitUser":                           `{"42":[20,10]}`,
		"ModelRequestRateLimitDistillationEnabled":            true,
		"ModelRequestRateLimitDistillationThreshold":          60,
		"ModelRequestRateLimitDistillationRPM":                10,
		"ModelRequestRateLimitDistillationPenaltyMinutes":     30,
		"ModelRequestRateLimitDistillationObservationMinutes": 1440,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	var response rateLimitTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success, response.Message)

	var options []model.Option
	require.NoError(t, model.DB.Find(&options).Error)
	optionValues := make(map[string]string, len(options))
	for _, option := range options {
		optionValues[option.Key] = option.Value
	}
	assert.Equal(t, "true", optionValues["ModelRequestRateLimitEnabled"])
	assert.Equal(t, "2", optionValues["ModelRequestRateLimitDurationMinutes"])
	assert.Equal(t, `{"42":[20,10]}`, optionValues["ModelRequestRateLimitUser"])
	assert.Equal(t, "1440", optionValues["ModelRequestRateLimitDistillationObservationMinutes"])

	total, success := setting.ResolveModelRequestRateLimit(42, "vip", 500, 300)
	assert.Equal(t, 20, total)
	assert.Equal(t, 10, success)
	assert.Equal(t, setting.DistillationRateLimitSettings{
		Enabled:            true,
		Threshold:          60,
		RPM:                10,
		PenaltyMinutes:     30,
		ObservationMinutes: 1440,
	}, setting.GetDistillationRateLimitSettings())
}

func TestUpdateRateLimitSettingsRejectsInvalidConfigurationBeforeWriting(t *testing.T) {
	setupRateLimitControllerTest(t)

	recorder := performRateLimitSettingsRequest(t, gin.H{
		"ModelRequestRateLimitEnabled":                        true,
		"ModelRequestRateLimitDurationMinutes":                1,
		"ModelRequestRateLimitCount":                          100,
		"ModelRequestRateLimitSuccessCount":                   100,
		"ModelRequestRateLimitGroup":                          `{}`,
		"ModelRequestRateLimitUser":                           `{}`,
		"ModelRequestRateLimitDistillationEnabled":            true,
		"ModelRequestRateLimitDistillationThreshold":          10,
		"ModelRequestRateLimitDistillationRPM":                10,
		"ModelRequestRateLimitDistillationPenaltyMinutes":     30,
		"ModelRequestRateLimitDistillationObservationMinutes": 1440,
	})

	var response rateLimitTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)

	var optionCount int64
	require.NoError(t, model.DB.Model(&model.Option{}).Count(&optionCount).Error)
	assert.Zero(t, optionCount)
}

func TestUpdateOptionRejectsInvalidUserRateLimitMap(t *testing.T) {
	setupRateLimitControllerTest(t)

	payload, err := common.Marshal(gin.H{
		"key":   "ModelRequestRateLimitUser",
		"value": `{"alice":[20,10]}`,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateOption(ctx)

	var response rateLimitTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)

	var optionCount int64
	require.NoError(t, model.DB.Model(&model.Option{}).Count(&optionCount).Error)
	assert.Zero(t, optionCount)
}
