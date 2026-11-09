package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type channelStatusTestResponse struct {
	LastUpdatedAt int64               `json:"last_updated_at"`
	WindowStart   int64               `json:"window_start"`
	BucketSeconds int64               `json:"bucket_seconds"`
	Counts        channelStatusCounts `json:"counts"`
	Items         []channelStatusItem `json:"items"`
}

func setupChannelStatusTest(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelStatusMonitor{}, &model.Log{}))
	previous := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previous)) })
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2,"spare":1}`))
	service.InvalidateChannelStatusSnapshot()
	t.Cleanup(service.InvalidateChannelStatusSnapshot)
}

func callChannelStatus(t *testing.T, handler gin.HandlerFunc, method string, role int, params gin.Params, body string) ticketResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = params
	ctx.Set("id", 1)
	ctx.Set("role", role)
	handler(ctx)
	var response ticketResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func addChannelStatusLog(t *testing.T, group, modelName string, logType int, createdAt int64, requestId string, statusCode int) {
	t.Helper()
	other := ""
	if logType == model.LogTypeError {
		other = `{"status_code":` + strconv.Itoa(statusCode) + `}`
	}
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		Type: logType, Group: group, ModelName: modelName, CreatedAt: createdAt,
		RequestId: requestId, UseTime: 2, Other: other,
	}).Error)
}

func TestChannelStatusCountsEachRequestOnceFromTheGroupLogs(t *testing.T) {
	setupChannelStatusTest(t)
	now := time.Now().Unix()
	for i := range 19 {
		addChannelStatusLog(t, "vip", "gpt-5", model.LogTypeConsume, now-60, "ok-"+strconv.Itoa(i), 0)
	}
	// A retried request that finally succeeded is one success.
	addChannelStatusLog(t, "vip", "gpt-5", model.LogTypeError, now-30, "retried", 503)
	addChannelStatusLog(t, "vip", "gpt-5", model.LogTypeConsume, now-29, "retried", 0)
	// The user received a rate limit: a failure.
	addChannelStatusLog(t, "vip", "claude", model.LogTypeError, now-10, "limited", 429)
	// Requests the user got wrong are left out, judged by the last error.
	addChannelStatusLog(t, "vip", "gpt-5", model.LogTypeError, now-20, "bad-input", 400)
	addChannelStatusLog(t, "vip", "gpt-5", model.LogTypeError, now-21, "bad-after-retry", 503)
	addChannelStatusLog(t, "vip", "gpt-5", model.LogTypeError, now-20, "bad-after-retry", 404)
	// Logs older than an hour and logs of unmonitored groups do not count.
	addChannelStatusLog(t, "vip", "gpt-5", model.LogTypeError, now-4000, "old", 503)
	addChannelStatusLog(t, "spare", "gpt-5", model.LogTypeError, now-10, "spare", 503)
	addChannelStatusLog(t, "default", "gpt-5", model.LogTypeConsume, now-10, "d-ok", 0)
	addChannelStatusLog(t, "default", "gpt-5", model.LogTypeError, now-10, "d-fail", 502)

	require.NoError(t, model.SaveChannelStatusMonitor(&model.ChannelStatusMonitor{Group: "vip", PublicName: "VIP", PublicEnabled: true}))
	require.NoError(t, model.SaveChannelStatusMonitor(&model.ChannelStatusMonitor{Group: "default", PublicName: "Internal"}))

	staffResponse := callChannelStatus(t, GetChannelStatus, http.MethodGet, common.RoleRootUser, nil, "")
	require.True(t, staffResponse.Success, staffResponse.Message)
	var staff channelStatusTestResponse
	require.NoError(t, common.Unmarshal(staffResponse.Data, &staff))
	require.Len(t, staff.Items, 2)
	vip, internal := staff.Items[0], staff.Items[1]
	assert.Equal(t, "vip", vip.Group)
	assert.Equal(t, int64(20), vip.SuccessCount)
	assert.Equal(t, int64(1), vip.FailureCount)
	require.NotNil(t, vip.SuccessRate)
	assert.InDelta(t, 100.0*20/21, *vip.SuccessRate, 0.001)
	assert.Equal(t, service.ChannelStatusOperational, vip.Status)
	require.NotNil(t, vip.AverageDurationMs)
	assert.InDelta(t, 2000, *vip.AverageDurationMs, 0.001)
	assert.Equal(t, service.ChannelStatusFailed, internal.Status, "1 of 2 requests failed")
	assert.Equal(t, channelStatusCounts{Operational: 1, Failed: 1}, staff.Counts)

	require.Len(t, vip.Timeline, 12)
	failureBucket := (now - 10 - staff.WindowStart) / staff.BucketSeconds
	assert.Equal(t, int64(1), vip.Timeline[failureBucket].FailureCount)
	assert.Equal(t, staff.WindowStart+failureBucket*staff.BucketSeconds, vip.Timeline[failureBucket].StartAt)

	userResponse := callChannelStatus(t, GetChannelStatus, http.MethodGet, common.RoleCommonUser, nil, "")
	var user channelStatusTestResponse
	require.NoError(t, common.Unmarshal(userResponse.Data, &user))
	require.Len(t, user.Items, 1, "users only see public monitors")
	assert.Equal(t, "VIP", user.Items[0].PublicName)
	assert.Empty(t, user.Items[0].Group, "users are not told the group")

	detail := callChannelStatus(t, GetChannelStatusDetail, http.MethodGet, common.RoleCommonUser, gin.Params{{Key: "id", Value: strconv.Itoa(vip.Id)}}, "")
	require.True(t, detail.Success, detail.Message)
	var detailData struct {
		Models []channelStatusModel `json:"models"`
	}
	require.NoError(t, common.Unmarshal(detail.Data, &detailData))
	require.Len(t, detailData.Models, 2)
	assert.Equal(t, "gpt-5", detailData.Models[0].Model, "busiest model first")
	assert.Equal(t, service.ChannelStatusFailed, detailData.Models[1].Status)
	hidden := callChannelStatus(t, GetChannelStatusDetail, http.MethodGet, common.RoleCommonUser, gin.Params{{Key: "id", Value: strconv.Itoa(internal.Id)}}, "")
	assert.Equal(t, model.ErrChannelStatusMonitorNotFound.Key, hidden.MessageKey)

	public := callChannelStatus(t, GetChannelStatusPublicSummary, http.MethodGet, 0, nil, "")
	assert.Contains(t, string(public.Data), `"public_name":"VIP"`)
	assert.NotContains(t, string(public.Data), "Internal")
}

func TestChannelStatusRatesTheShareOfSuccessfulRequests(t *testing.T) {
	tests := []struct {
		success, failure int64
		status           string
	}{
		{0, 0, service.ChannelStatusUnknown},
		{95, 5, service.ChannelStatusOperational},
		{94, 6, service.ChannelStatusDegraded},
		{80, 20, service.ChannelStatusDegraded},
		{79, 21, service.ChannelStatusFailed},
	}
	for _, tt := range tests {
		figures := service.ChannelStatusFiguresOf(service.ChannelStatusTally{Success: tt.success, Failure: tt.failure})
		assert.Equal(t, tt.status, figures.Status, "%d/%d", tt.success, tt.failure)
	}
}

func TestChannelStatusMonitorsAreValidatedPerGroup(t *testing.T) {
	setupChannelStatusTest(t)
	create := func(body string) ticketResponse {
		return callChannelStatus(t, CreateChannelStatusMonitor, http.MethodPost, common.RoleRootUser, nil, body)
	}

	assert.Equal(t, "This group does not exist", create(`{"group":"ghost","public_name":"Ghost"}`).MessageKey)
	assert.Equal(t, "Display name is required", create(`{"group":"vip","public_name":"  "}`).MessageKey)
	assert.Equal(t, "Name color must use #RRGGBB format", create(`{"group":"vip","public_name":"VIP","name_color":"red"}`).MessageKey)
	for _, threshold := range []string{"-1", "101"} {
		assert.Equal(t, "Cache hit threshold must be from 1 to 100, or empty", create(`{"group":"vip","public_name":"VIP","cache_hit_threshold":`+threshold+`}`).MessageKey)
	}

	created := create(`{"group":"vip","public_name":"VIP","name_color":"#12ab3c","public_enabled":true,"cache_hit_threshold":80}`)
	require.True(t, created.Success, created.Message)
	var monitor model.ChannelStatusMonitor
	require.NoError(t, common.Unmarshal(created.Data, &monitor))
	assert.Equal(t, "#12AB3C", monitor.NameColor)
	assert.Equal(t, 80, monitor.CacheHitThreshold)
	assert.Equal(t, model.ErrChannelStatusGroupTaken.Key, create(`{"group":"vip","public_name":"Again"}`).MessageKey)

	idParams := gin.Params{{Key: "id", Value: strconv.Itoa(monitor.Id)}}
	updated := callChannelStatus(t, UpdateChannelStatusMonitor, http.MethodPut, common.RoleRootUser, idParams, `{"group":"default","public_name":"Standard","public_enabled":false}`)
	require.True(t, updated.Success, updated.Message)
	stored, err := model.GetChannelStatusMonitor(monitor.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "default", stored.Group)
	assert.False(t, stored.PublicEnabled)
	assert.Zero(t, stored.CacheHitThreshold)

	deleted := callChannelStatus(t, DeleteChannelStatusMonitor, http.MethodDelete, common.RoleRootUser, idParams, "")
	require.True(t, deleted.Success, deleted.Message)
	again := callChannelStatus(t, DeleteChannelStatusMonitor, http.MethodDelete, common.RoleRootUser, idParams, "")
	assert.Equal(t, model.ErrChannelStatusMonitorNotFound.Key, again.MessageKey)
}

func addChannelStatusCacheLog(t *testing.T, group string, createdAt int64, promptTokens int, other string) {
	t.Helper()
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		Type: model.LogTypeConsume, Group: group, ModelName: "claude", CreatedAt: createdAt,
		PromptTokens: promptTokens, UseTime: 1, Other: other,
	}).Error)
}

func TestChannelStatusSharesRequestsThatReachTheCacheHitThreshold(t *testing.T) {
	setupChannelStatusTest(t)
	now := time.Now().Unix()
	addChannelStatusCacheLog(t, "vip", now-60, 1000, `{"cache_tokens":600}`) // 60%
	addChannelStatusCacheLog(t, "vip", now-60, 1000, `{"cache_tokens":900}`) // 90%
	// Claude usage counts cache reads and writes as input: 300 of 1000 is 30%.
	addChannelStatusCacheLog(t, "vip", now-60, 200, `{"cache_tokens":300,"cache_creation_tokens":500,"claude":true}`)
	addChannelStatusCacheLog(t, "vip", now-60, 1000, `{"cache_tokens":0}`) // 0%
	// A request without input tokens, such as an image, is not counted.
	addChannelStatusCacheLog(t, "vip", now-60, 0, "")
	// A bucket that ended long ago, counted once and kept: 40%.
	addChannelStatusCacheLog(t, "vip", now-1800, 1000, `{"cache_tokens":400}`)
	addChannelStatusCacheLog(t, "default", now-60, 1000, `{"cache_tokens":900}`)

	vip := &model.ChannelStatusMonitor{Group: "vip", PublicName: "VIP", PublicEnabled: true, CacheHitThreshold: 50}
	require.NoError(t, model.SaveChannelStatusMonitor(vip))
	require.NoError(t, model.SaveChannelStatusMonitor(&model.ChannelStatusMonitor{Group: "default", PublicName: "Standard", PublicEnabled: true}))
	items := func() []channelStatusItem {
		response := callChannelStatus(t, GetChannelStatus, http.MethodGet, common.RoleCommonUser, nil, "")
		require.True(t, response.Success, response.Message)
		var status channelStatusTestResponse
		require.NoError(t, common.Unmarshal(response.Data, &status))
		require.Len(t, status.Items, 2)
		return status.Items
	}

	listed := items()
	assert.Equal(t, 50, listed[0].CacheHitThreshold, "users see the cache figures of public monitors")
	assert.Equal(t, int64(5), listed[0].CacheRequestCount)
	assert.Equal(t, int64(2), listed[0].CacheHitCount, "90% and 60% reach 50%")
	require.NotNil(t, listed[0].CacheHitShare)
	assert.InDelta(t, 40.0, *listed[0].CacheHitShare, 0.001)
	assert.Zero(t, listed[1].CacheHitThreshold)
	assert.Nil(t, listed[1].CacheHitShare, "a monitor without a threshold shows no cache figures")
	assert.Zero(t, listed[1].CacheRequestCount)

	// A lower threshold recounts the ended bucket too: its 40% now counts.
	updated := callChannelStatus(t, UpdateChannelStatusMonitor, http.MethodPut, common.RoleRootUser,
		gin.Params{{Key: "id", Value: strconv.Itoa(vip.Id)}}, `{"group":"vip","public_name":"VIP","public_enabled":true,"cache_hit_threshold":30}`)
	require.True(t, updated.Success, updated.Message)
	listed = items()
	assert.Equal(t, int64(4), listed[0].CacheHitCount, "90%, 60%, 40% and 30% reach 30%")
	require.NotNil(t, listed[0].CacheHitShare)
	assert.InDelta(t, 80.0, *listed[0].CacheHitShare, 0.001)
}
