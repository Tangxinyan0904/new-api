package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
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

func TestListSelfDistillationViolationRecordsIsolatesAuthenticatedUserAndPaginates(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.DistillationViolationRecord{}))
	records := []model.DistillationViolationRecord{
		{
			UserId: 301, CycleStartedAt: 1000, TriggeredAt: 1000,
			RequestCount: 200, DetectionThreshold: 200, PenaltyRPM: 10,
			Action: model.DistillationViolationActionTemporaryLimit, EffectiveUntil: 1600,
		},
		{
			UserId: 301, CycleStartedAt: 1000, TriggeredAt: 1700,
			RequestCount: 200, DetectionThreshold: 200, PenaltyRPM: 10,
			Action: model.DistillationViolationActionPermanentBan,
		},
		{
			UserId: 999, CycleStartedAt: 2000, TriggeredAt: 2000,
			RequestCount: 300, DetectionThreshold: 300, PenaltyRPM: 20,
			Action: model.DistillationViolationActionTemporaryLimit, EffectiveUntil: 2600,
		},
	}
	require.NoError(t, db.Create(&records).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/distillation/violations/self?p=1&page_size=1&user_id=999", nil)
	ctx.Set("id", 301)

	ListSelfDistillationViolationRecords(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Total int                                  `json:"total"`
			Items []*model.DistillationViolationRecord `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, records[1].Id, response.Data.Items[0].Id)
	assert.Equal(t, model.DistillationViolationActionPermanentBan, response.Data.Items[0].Action)
	assert.NotContains(t, recorder.Body.String(), "user_id")
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

func TestListDistillationPenaltiesReturnsPaginatedSearchResultsAndExcludesExpiredRows(t *testing.T) {
	setupRateLimitControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.DistillationPenalty{}, &model.DistillationViolationRecord{}))

	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 41, Username: "alice-one", Password: "password", Status: common.UserStatusEnabled, AffCode: "u41"},
		{Id: 42, Username: "alice-two", Password: "password", Status: common.UserStatusEnabled, AffCode: "u42"},
		{Id: 43, Username: "expired-alice", Password: "password", Status: common.UserStatusEnabled, AffCode: "u43"},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.DistillationPenalty{
		{
			UserId:                41,
			FirstTriggeredAt:      now - 60,
			TemporaryLimitedUntil: now + 600,
			ObservationUntil:      now + 3600,
			UpdatedAt:             now - 10,
		},
		{
			UserId:                42,
			FirstTriggeredAt:      now - 120,
			TemporaryLimitedUntil: now - 60,
			ObservationUntil:      now + 7200,
			PermanentlyBannedAt:   now - 30,
			UpdatedAt:             now,
		},
		{
			UserId:                43,
			FirstTriggeredAt:      now - 7200,
			TemporaryLimitedUntil: now - 7100,
			ObservationUntil:      now - 1,
			UpdatedAt:             now + 10,
		},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/rate-limit/distillation/penalties?p=1&page_size=1&keyword=alice", nil)

	ListDistillationPenalties(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Page     int                                  `json:"page"`
			PageSize int                                  `json:"page_size"`
			Total    int                                  `json:"total"`
			Items    []*model.DistillationPenaltyListItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Page)
	assert.Equal(t, 1, response.Data.PageSize)
	assert.Equal(t, 2, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, 42, response.Data.Items[0].UserId)
	assert.Equal(t, "alice-two", response.Data.Items[0].Username)
	assert.Equal(t, model.DistillationPenaltyPhasePermanent, response.Data.Items[0].Phase)

	idRecorder := httptest.NewRecorder()
	idCtx, _ := gin.CreateTestContext(idRecorder)
	idCtx.Request = httptest.NewRequest(http.MethodGet, "/api/rate-limit/distillation/penalties?p=1&page_size=10&keyword=41", nil)

	ListDistillationPenalties(idCtx)

	var idResponse struct {
		Success bool `json:"success"`
		Data    struct {
			Total int                                  `json:"total"`
			Items []*model.DistillationPenaltyListItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(idRecorder.Body.Bytes(), &idResponse))
	assert.True(t, idResponse.Success)
	assert.Equal(t, 1, idResponse.Data.Total)
	require.Len(t, idResponse.Data.Items, 1)
	assert.Equal(t, 41, idResponse.Data.Items[0].UserId)
}

func TestClearDistillationPenaltyIsIdempotentInvalidatesCachedBanAndRecordsAudit(t *testing.T) {
	setupRateLimitControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.DistillationPenalty{}, &model.DistillationViolationRecord{}))

	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 1, Username: "root-admin", Password: "password", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AffCode: "root"},
		{Id: 51, Username: "limited-user", Password: "password", Status: common.UserStatusEnabled, AffCode: "u51"},
	}).Error)
	require.NoError(t, model.DB.Create(&model.DistillationPenalty{
		UserId:                51,
		FirstTriggeredAt:      now - 120,
		TemporaryLimitedUntil: now - 60,
		ObservationUntil:      now + 3600,
		PermanentlyBannedAt:   now - 30,
	}).Error)

	relayRecorder := httptest.NewRecorder()
	relayCtx, _ := gin.CreateTestContext(relayRecorder)
	relayCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	banError := service.CheckDistillationRateLimit(relayCtx, &relaycommon.RelayInfo{UserId: 51, IsStream: true})
	require.NotNil(t, banError)

	performClear := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/rate-limit/distillation/penalties/51", nil)
		ctx.Params = gin.Params{{Key: "user_id", Value: "51"}}
		ctx.Set("id", 1)
		ctx.Set("username", "root-admin")
		ctx.Set("role", common.RoleRootUser)
		ClearDistillationPenalty(ctx)
		return recorder
	}

	first := performClear()
	second := performClear()

	for _, recorder := range []*httptest.ResponseRecorder{first, second} {
		var response rateLimitTestResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.True(t, response.Success, response.Message)
	}
	var penaltyCount int64
	require.NoError(t, model.DB.Model(&model.DistillationPenalty{}).Where("user_id = ?", 51).Count(&penaltyCount).Error)
	assert.Zero(t, penaltyCount)

	afterClearError := service.CheckDistillationRateLimit(relayCtx, &relaycommon.RelayInfo{UserId: 51, IsStream: true})
	assert.Nil(t, afterClearError)

	var auditLogs []model.Log
	require.NoError(t, model.DB.Where("type = ?", model.LogTypeManage).Order("id ASC").Find(&auditLogs).Error)
	require.Len(t, auditLogs, 2)
	for _, auditLog := range auditLogs {
		assert.Contains(t, auditLog.Content, "Cleared distillation penalty for user 51")
		other, err := common.StrToMap(auditLog.Other)
		require.NoError(t, err)
		op, ok := other["op"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "rate_limit.distillation_clear", op["action"])
		params, ok := op["params"].(map[string]interface{})
		require.True(t, ok)
		assert.EqualValues(t, 51, params["target_user_id"])
	}
}

func TestClearDistillationPenaltyResetsDetectionWindow(t *testing.T) {
	setupRateLimitControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.DistillationPenalty{}, &model.DistillationViolationRecord{}))
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1,
		Username: "root-admin",
		Password: "password",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "root",
	}).Error)
	require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationEnabled", "true"))
	require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationThreshold", "2"))
	require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationRPM", "1"))
	require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationPenaltyMinutes", "10"))
	require.NoError(t, setting.UpdateDistillationRateLimitOption("ModelRequestRateLimitDistillationObservationMinutes", "60"))

	relayRecorder := httptest.NewRecorder()
	relayCtx, _ := gin.CreateTestContext(relayRecorder)
	relayCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{UserId: 52, IsStream: false}
	require.Nil(t, service.CheckDistillationRateLimit(relayCtx, relayInfo))

	clearRecorder := httptest.NewRecorder()
	clearCtx, _ := gin.CreateTestContext(clearRecorder)
	clearCtx.Request = httptest.NewRequest(http.MethodDelete, "/api/rate-limit/distillation/penalties/52", nil)
	clearCtx.Params = gin.Params{{Key: "user_id", Value: "52"}}
	clearCtx.Set("id", 1)
	clearCtx.Set("username", "root-admin")
	clearCtx.Set("role", common.RoleRootUser)
	ClearDistillationPenalty(clearCtx)

	require.Nil(t, service.CheckDistillationRateLimit(relayCtx, relayInfo))
	var penaltyCount int64
	require.NoError(t, model.DB.Model(&model.DistillationPenalty{}).Where("user_id = ?", 52).Count(&penaltyCount).Error)
	assert.Zero(t, penaltyCount)

	require.Nil(t, service.CheckDistillationRateLimit(relayCtx, relayInfo))
	require.NoError(t, model.DB.Model(&model.DistillationPenalty{}).Where("user_id = ?", 52).Count(&penaltyCount).Error)
	assert.EqualValues(t, 1, penaltyCount)
}

func TestClearDistillationPenaltyRejectsNonPositiveUserID(t *testing.T) {
	setupRateLimitControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.DistillationPenalty{}, &model.DistillationViolationRecord{}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/rate-limit/distillation/penalties/0", nil)
	ctx.Params = gin.Params{{Key: "user_id", Value: "0"}}

	ClearDistillationPenalty(ctx)

	var response rateLimitTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}
