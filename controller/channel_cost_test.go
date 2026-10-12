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
	DefaultRatio float64              `json:"default_ratio"`
	Channels     []channelCostChannel `json:"channels"`
}

func setupChannelCostTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.ChannelCostHourly{}, &model.Option{}, &model.AuditLog{}))
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, model.UpdateOptionsBulk(map[string]string{
			channelCostDefaultRatioOption:  "1",
			channelCostChannelRatiosOption: "{}",
		}))
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

func TestRebuildChannelCostHourNetsRefundsAndReplacesTheBucket(t *testing.T) {
	db := setupChannelCostTestDB(t)
	const hour int64 = 1_790_000_000 - 1_790_000_000%3600
	require.NoError(t, db.Create(&[]model.Log{
		{ChannelId: 1, Type: model.LogTypeConsume, Quota: 100, CreatedAt: hour},
		{ChannelId: 1, Type: model.LogTypeConsume, Quota: 50, CreatedAt: hour + 3599},
		{ChannelId: 1, Type: model.LogTypeRefund, Quota: 30, CreatedAt: hour + 10},
		{ChannelId: 2, Type: model.LogTypeConsume, Quota: 70, CreatedAt: hour + 20},
		{ChannelId: 1, Type: model.LogTypeTopup, Quota: 5000, CreatedAt: hour + 30},
		{ChannelId: 0, Type: model.LogTypeConsume, Quota: 10, CreatedAt: hour + 40},
		{ChannelId: 1, Type: model.LogTypeConsume, Quota: 999, CreatedAt: hour + 3600},
	}).Error)

	require.NoError(t, model.RebuildChannelCostHour(hour))
	require.NoError(t, db.Create(&model.Log{ChannelId: 2, Type: model.LogTypeConsume, Quota: 5, CreatedAt: hour + 50}).Error)
	require.NoError(t, model.RebuildChannelCostHour(hour))

	var buckets []model.ChannelCostHourly
	require.NoError(t, db.Order("channel_id").Find(&buckets).Error)
	require.Len(t, buckets, 2, "a rebuild replaces the hour instead of adding to it")
	assert.Equal(t, model.ChannelCostHourly{Id: buckets[0].Id, BucketStart: hour, ChannelId: 1, Quota: 120}, buckets[0])
	assert.Equal(t, model.ChannelCostHourly{Id: buckets[1].Id, BucketStart: hour, ChannelId: 2, Quota: 75}, buckets[1])
}

func TestGetChannelCostStatsSplitsRangesAndKeepsDeletedChannels(t *testing.T) {
	db := setupChannelCostTestDB(t)
	now := common.GetTimestamp()
	today := now - now%3600 - 5*3600
	yesterday := today - 24*3600
	week := today - 6*24*3600
	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 1, Name: "openai-main", Type: 1, Status: 1},
		{Id: 2, Name: "claude-backup", Type: 14, Status: 2},
		{Id: 3, Name: "idle", Type: 1, Status: 1},
	}).Error)
	require.NoError(t, db.Create(&[]model.ChannelCostHourly{
		{BucketStart: today, ChannelId: 1, Quota: 100},
		{BucketStart: today + 3600, ChannelId: 1, Quota: 20},
		{BucketStart: yesterday, ChannelId: 1, Quota: 300},
		{BucketStart: week, ChannelId: 1, Quota: 1000},
		{BucketStart: week - 3600, ChannelId: 1, Quota: 9999},
		{BucketStart: yesterday + 3600, ChannelId: 2, Quota: 50},
		{BucketStart: today, ChannelId: 9, Quota: 7},
	}).Error)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		channelCostDefaultRatioOption:  "0.6",
		channelCostChannelRatiosOption: `{"1":0.4}`,
	}))

	target := "/api/channel-cost/stats?today_start=" + strconv.FormatInt(today, 10) +
		"&yesterday_start=" + strconv.FormatInt(yesterday, 10) + "&week_start=" + strconv.FormatInt(week, 10)
	response := callChannelCost(t, GetChannelCostStats, http.MethodGet, target, nil, "")
	require.True(t, response.Success, string(response.Data))
	var stats channelCostStats
	require.NoError(t, common.Unmarshal(response.Data, &stats))

	assert.Equal(t, 0.6, stats.DefaultRatio)
	require.Len(t, stats.Channels, 4)
	first := stats.Channels[0]
	assert.Equal(t, 1, first.Id)
	assert.Equal(t, [3]int64{120, 300, 1420}, [3]int64{first.TodayQuota, first.YesterdayQuota, first.WeekQuota})
	require.NotNil(t, first.Ratio)
	assert.Equal(t, 0.4, *first.Ratio)

	assert.Equal(t, 2, stats.Channels[1].Id)
	assert.Equal(t, [3]int64{0, 50, 50}, [3]int64{stats.Channels[1].TodayQuota, stats.Channels[1].YesterdayQuota, stats.Channels[1].WeekQuota})
	assert.Nil(t, stats.Channels[1].Ratio, "channels without their own ratio use the default")

	assert.Equal(t, 9, stats.Channels[2].Id)
	assert.True(t, stats.Channels[2].Deleted)
	assert.Equal(t, int64(7), stats.Channels[2].TodayQuota)

	assert.Equal(t, 3, stats.Channels[3].Id, "unused channels are listed after used ones")
	assert.Zero(t, stats.Channels[3].WeekQuota)
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

func TestUpdateChannelCostRatios(t *testing.T) {
	db := setupChannelCostTestDB(t)
	require.NoError(t, db.Create(&model.Channel{Id: 5, Name: "upstream", Type: 1, Status: 1}).Error)
	idParam := gin.Params{{Key: "id", Value: "5"}}

	ok := callChannelCost(t, UpdateChannelCostDefaultRatio, http.MethodPut, "/", nil, `{"ratio":0.35}`)
	require.True(t, ok.Success)
	assert.Equal(t, 0.35, operation_setting.GetChannelCostSetting().DefaultRatio)

	set := callChannelCost(t, UpdateChannelCostRatio, http.MethodPut, "/", idParam, `{"ratio":0.2}`)
	require.True(t, set.Success)
	assert.Equal(t, map[int]float64{5: 0.2}, operation_setting.GetChannelCostSetting().ChannelRatios)

	cleared := callChannelCost(t, UpdateChannelCostRatio, http.MethodPut, "/", idParam, `{"ratio":null}`)
	require.True(t, cleared.Success)
	assert.Empty(t, operation_setting.GetChannelCostSetting().ChannelRatios)

	for body, key := range map[string]string{
		`{"ratio":-0.1}`: "Cost ratio must be between 0 and {{max}}",
		`{"ratio":101}`:  "Cost ratio must be between 0 and {{max}}",
		`{}`:             "Invalid parameters",
	} {
		response := callChannelCost(t, UpdateChannelCostDefaultRatio, http.MethodPut, "/", nil, body)
		assert.False(t, response.Success, body)
		assert.Equal(t, key, response.MessageKey, body)
	}
	assert.Equal(t, 0.35, operation_setting.GetChannelCostSetting().DefaultRatio, "rejected input keeps the saved ratio")

	missing := callChannelCost(t, UpdateChannelCostRatio, http.MethodPut, "/", gin.Params{{Key: "id", Value: "404"}}, `{"ratio":0.5}`)
	assert.False(t, missing.Success)
	assert.Equal(t, "Channel not found", missing.MessageKey)

	var audits int64
	require.NoError(t, db.Model(&model.AuditLog{}).Where("action = ?", "option.update").Count(&audits).Error)
	assert.Equal(t, int64(3), audits, "each saved change is audited")
}
