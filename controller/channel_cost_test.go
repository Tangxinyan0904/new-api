package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type channelCostResponse struct {
	Success    bool            `json:"success"`
	MessageKey string          `json:"message_key"`
	Data       json.RawMessage `json:"data"`
}

type channelCostStats struct {
	Groups []channelCostGroup `json:"groups"`
}

// setupChannelCostTestDB also configures the group ratios "default" at 1 and
// "vip" at 0.5.
func setupChannelCostTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.ChannelCostHourly{}, &model.Option{}, &model.AuditLog{}))
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMapRWMutex.Unlock()
	savedGroupRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":0.5}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatios))
		require.NoError(t, model.UpdateOptionsBulk(map[string]string{channelCostChannelRatiosOption: "{}"}))
	})
	return db
}

func callChannelCost(t *testing.T, handler gin.HandlerFunc, method, target string, params gin.Params, body string) channelCostResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Params = params
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)
	handler(ctx)
	var response channelCostResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestRebuildChannelCostHourSplitsGroupsAtTheChargedRatio(t *testing.T) {
	db := setupChannelCostTestDB(t)
	const hour int64 = 1_790_000_000 - 1_790_000_000%3600
	require.NoError(t, db.Create(&[]model.Log{
		{ChannelId: 1, Group: "vip", Type: model.LogTypeConsume, Quota: 100, CreatedAt: hour, Other: `{"group_ratio":0.5,"user_group_ratio":-1}`},
		// A user group special ratio is what the request was charged at.
		{ChannelId: 1, Group: "vip", Type: model.LogTypeConsume, Quota: 40, CreatedAt: hour + 3599, Other: `{"group_ratio":0.4,"user_group_ratio":0.4}`},
		{ChannelId: 1, Group: "vip", Type: model.LogTypeRefund, Quota: 50, CreatedAt: hour + 10, Other: `{"group_ratio":0.5}`},
		{ChannelId: 1, Group: "default", Type: model.LogTypeConsume, Quota: 60, CreatedAt: hour + 20, Other: `{"group_ratio":"0.25"}`},
		// Without a recorded ratio: the group's current ratio, or 1 if it has none.
		{ChannelId: 2, Group: "vip", Type: model.LogTypeConsume, Quota: 70, CreatedAt: hour + 30},
		{ChannelId: 2, Group: "retired", Type: model.LogTypeConsume, Quota: 9, CreatedAt: hour + 40, Other: "{}"},
		{ChannelId: 1, Group: "vip", Type: model.LogTypeTopup, Quota: 5000, CreatedAt: hour + 50},
		{ChannelId: 0, Group: "vip", Type: model.LogTypeConsume, Quota: 10, CreatedAt: hour + 60},
		{ChannelId: 1, Group: "vip", Type: model.LogTypeConsume, Quota: 999, CreatedAt: hour + 3600},
	}).Error)

	require.NoError(t, model.RebuildChannelCostHour(hour))
	require.NoError(t, db.Create(&model.Log{ChannelId: 2, Group: "vip", Type: model.LogTypeConsume, Quota: 5, CreatedAt: hour + 70}).Error)
	require.NoError(t, model.RebuildChannelCostHour(hour))

	var buckets []model.ChannelCostHourly
	require.NoError(t, db.Order("group_name, channel_id").Find(&buckets).Error)
	got := make([]model.ChannelCostHourly, 0, len(buckets))
	for _, bucket := range buckets {
		assert.Equal(t, hour, bucket.BucketStart)
		bucket.Id, bucket.BucketStart = 0, 0
		got = append(got, bucket)
	}
	assert.Equal(t, []model.ChannelCostHourly{
		{Group: "default", ChannelId: 1, Quota: 60, BaseQuota: 240},
		{Group: "retired", ChannelId: 2, Quota: 9, BaseQuota: 9},
		{Group: "vip", ChannelId: 1, Quota: 90, BaseQuota: 200},
		{Group: "vip", ChannelId: 2, Quota: 75, BaseQuota: 150},
	}, got, "a rebuild replaces the hour instead of adding to it")
}

func TestGetChannelCostStatsListsChannelsUnderEachGroup(t *testing.T) {
	db := setupChannelCostTestDB(t)
	now := common.GetTimestamp()
	today := now - now%3600 - 5*3600
	yesterday := today - 24*3600
	week := today - 6*24*3600
	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 1, Name: "openai-main", Type: 1, Status: 1, Group: "default,vip"},
		{Id: 2, Name: "claude-backup", Type: 14, Status: 2, Group: "vip"},
		{Id: 3, Name: "idle", Type: 1, Status: 1, Group: "default"},
	}).Error)
	require.NoError(t, db.Create(&[]model.ChannelCostHourly{
		{BucketStart: today, Group: "vip", ChannelId: 1, Quota: 100, BaseQuota: 200},
		{BucketStart: yesterday, Group: "vip", ChannelId: 1, Quota: 300, BaseQuota: 600},
		{BucketStart: week, Group: "vip", ChannelId: 1, Quota: 1000, BaseQuota: 2000},
		{BucketStart: week - 3600, Group: "vip", ChannelId: 1, Quota: 9999, BaseQuota: 9999},
		{BucketStart: yesterday + 3600, Group: "vip", ChannelId: 2, Quota: 50, BaseQuota: 100},
		{BucketStart: today + 3600, Group: "default", ChannelId: 1, Quota: 20, BaseQuota: 20},
		// Channel 2 has since left "default"; its usage there still counts.
		{BucketStart: week, Group: "default", ChannelId: 2, Quota: 5, BaseQuota: 5},
		{BucketStart: today, Group: "legacy", ChannelId: 9, Quota: 7, BaseQuota: 7},
	}).Error)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{channelCostChannelRatiosOption: `{"1":0.4}`}))

	target := "/api/channel-cost/stats?today_start=" + strconv.FormatInt(today, 10) +
		"&yesterday_start=" + strconv.FormatInt(yesterday, 10) + "&week_start=" + strconv.FormatInt(week, 10)
	response := callChannelCost(t, GetChannelCostStats, http.MethodGet, target, nil, "")
	require.True(t, response.Success, string(response.Data))
	var stats channelCostStats
	require.NoError(t, common.Unmarshal(response.Data, &stats))

	ratio := func(value float64) *float64 { return &value }
	assert.Equal(t, []channelCostGroup{
		{Name: "vip", Ratio: ratio(0.5), Channels: []channelCostChannel{
			{
				Id: 1, Name: "openai-main", Type: 1, Status: 1, Ratio: ratio(0.4),
				Today:     model.ChannelCostUsage{Quota: 100, BaseQuota: 200},
				Yesterday: model.ChannelCostUsage{Quota: 300, BaseQuota: 600},
				Week:      model.ChannelCostUsage{Quota: 1400, BaseQuota: 2800},
			},
			{
				Id: 2, Name: "claude-backup", Type: 14, Status: 2,
				Yesterday: model.ChannelCostUsage{Quota: 50, BaseQuota: 100},
				Week:      model.ChannelCostUsage{Quota: 50, BaseQuota: 100},
			},
		}},
		{Name: "default", Ratio: ratio(1), Channels: []channelCostChannel{
			{
				Id: 1, Name: "openai-main", Type: 1, Status: 1, Ratio: ratio(0.4),
				Today: model.ChannelCostUsage{Quota: 20, BaseQuota: 20},
				Week:  model.ChannelCostUsage{Quota: 20, BaseQuota: 20},
			},
			{
				Id: 2, Name: "claude-backup", Type: 14, Status: 2,
				Week: model.ChannelCostUsage{Quota: 5, BaseQuota: 5},
			},
			{Id: 3, Name: "idle", Type: 1, Status: 1},
		}},
		{Name: "legacy", Channels: []channelCostChannel{
			{
				Id: 9, Deleted: true,
				Today: model.ChannelCostUsage{Quota: 7, BaseQuota: 7},
				Week:  model.ChannelCostUsage{Quota: 7, BaseQuota: 7},
			},
		}},
	}, stats.Groups)
}

func TestGetChannelCostStatsRejectsInvalidRanges(t *testing.T) {
	setupChannelCostTestDB(t)
	now := common.GetTimestamp()
	day := int64(24 * 3600)
	tests := map[string][3]int64{
		"yesterday after today":  {now - day, now, now - 2*day},
		"week after yesterday":   {now - day, now - 2*day, now - day},
		"today in the future":    {now + day, now, now - 6*day},
		"range beyond retention": {now - day, now - 2*day, now - 30*day},
	}
	for name, starts := range tests {
		target := "/api/channel-cost/stats?today_start=" + strconv.FormatInt(starts[0], 10) +
			"&yesterday_start=" + strconv.FormatInt(starts[1], 10) + "&week_start=" + strconv.FormatInt(starts[2], 10)
		response := callChannelCost(t, GetChannelCostStats, http.MethodGet, target, nil, "")
		assert.False(t, response.Success, name)
		assert.Equal(t, "Invalid parameters", response.MessageKey, name)
	}
}

func TestUpdateChannelCostRatio(t *testing.T) {
	db := setupChannelCostTestDB(t)
	require.NoError(t, db.Create(&model.Channel{Id: 5, Name: "upstream", Type: 1, Status: 1}).Error)
	idParam := gin.Params{{Key: "id", Value: "5"}}

	set := callChannelCost(t, UpdateChannelCostRatio, http.MethodPut, "/", idParam, `{"ratio":0.08}`)
	require.True(t, set.Success)
	assert.Equal(t, map[int]float64{5: 0.08}, operation_setting.GetChannelCostSetting().ChannelRatios)

	for body, key := range map[string]string{
		`{"ratio":-0.1}`: "Cost ratio must be between 0 and {{max}}",
		`{"ratio":101}`:  "Cost ratio must be between 0 and {{max}}",
		`not json`:       "Invalid parameters",
	} {
		response := callChannelCost(t, UpdateChannelCostRatio, http.MethodPut, "/", idParam, body)
		assert.False(t, response.Success, body)
		assert.Equal(t, key, response.MessageKey, body)
	}
	assert.Equal(t, map[int]float64{5: 0.08}, operation_setting.GetChannelCostSetting().ChannelRatios, "rejected input keeps the saved ratio")

	cleared := callChannelCost(t, UpdateChannelCostRatio, http.MethodPut, "/", idParam, `{"ratio":null}`)
	require.True(t, cleared.Success)
	assert.Empty(t, operation_setting.GetChannelCostSetting().ChannelRatios)

	missing := callChannelCost(t, UpdateChannelCostRatio, http.MethodPut, "/", gin.Params{{Key: "id", Value: "404"}}, `{"ratio":0.5}`)
	assert.False(t, missing.Success)
	assert.Equal(t, "Channel not found", missing.MessageKey)

	var audits int64
	require.NoError(t, db.Model(&model.AuditLog{}).Where("action = ?", "option.update").Count(&audits).Error)
	assert.Equal(t, int64(2), audits, "each saved change is audited")
}
