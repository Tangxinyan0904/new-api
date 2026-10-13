package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type usageActivityResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

func setupUsageActivityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.Token{}, &model.SystemInstance{}, &model.QuotaData{}))
	return db
}

func callUsageActivity(t *testing.T, handler gin.HandlerFunc, userId int, target string) json.RawMessage {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	ctx.Set("id", userId)
	handler(ctx)
	var response usageActivityResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, recorder.Body.String())
	return response.Data
}

func TestRelayActivityAddsLiveNodesAndCountsTheLastMinute(t *testing.T) {
	db := setupUsageActivityTestDB(t)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&[]model.Token{
		{Id: 10, UserId: 1, Key: "key-10", Name: "main"},
		{Id: 11, UserId: 1, Key: "key-11", Name: "idle"},
		{Id: 20, UserId: 2, Key: "key-20", Name: "other user"},
	}).Error)

	// This node: user 1 is running two requests on key 10, user 2 one on key 20.
	ends := []func(){
		service.BeginRelayRequest(1, 10),
		service.BeginRelayRequest(1, 10),
		service.BeginRelayRequest(2, 20),
	}
	t.Cleanup(func() {
		for _, end := range ends {
			end()
		}
	})
	// This node's own report must not be counted twice.
	require.NoError(t, service.ReportCurrentSystemInstance())
	require.NoError(t, model.UpsertSystemInstance("node-b", map[string]any{
		"relay_activity": service.RelayActivityCounts{Total: 3, Users: map[int]int64{1: 1, 3: 2}, Tokens: map[int]int64{10: 1, 30: 2}},
	}, now-600, now-5))
	require.NoError(t, model.UpsertSystemInstance("node-gone", map[string]any{
		"relay_activity": service.RelayActivityCounts{Total: 50, Users: map[int]int64{1: 50}, Tokens: map[int]int64{10: 50}},
	}, now-6000, now-model.SystemInstanceStaleAfterSeconds-1))

	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 1, TokenId: 10, Type: model.LogTypeConsume, CreatedAt: now - 5},
		{UserId: 1, TokenId: 10, Type: model.LogTypeConsume, CreatedAt: now - 30},
		{UserId: 1, TokenId: 11, Type: model.LogTypeConsume, CreatedAt: now - 59},
		{UserId: 1, TokenId: 10, Type: model.LogTypeConsume, CreatedAt: now - 120},
		{UserId: 1, TokenId: 10, Type: model.LogTypeError, CreatedAt: now - 5},
		{UserId: 2, TokenId: 20, Type: model.LogTypeConsume, CreatedAt: now - 5},
	}).Error)

	var self relayActivityStat
	require.NoError(t, common.Unmarshal(callUsageActivity(t, GetSelfRelayActivity, 1, "/api/data/self/activity"), &self))
	assert.Equal(t, relayActivityStat{Concurrency: 3, Rpm: 3}, self)

	var all relayActivityStat
	require.NoError(t, common.Unmarshal(callUsageActivity(t, GetAllRelayActivity, 1, "/api/data/activity"), &all))
	assert.Equal(t, relayActivityStat{Concurrency: 6, Rpm: 4}, all)

	var tokens []tokenRelayActivityStat
	require.NoError(t, common.Unmarshal(callUsageActivity(t, GetTokenRelayActivity, 1, "/api/token/activity"), &tokens))
	assert.Equal(t, []tokenRelayActivityStat{
		{TokenId: 10, relayActivityStat: relayActivityStat{Concurrency: 3, Rpm: 2}},
		{TokenId: 11, relayActivityStat: relayActivityStat{Rpm: 1}},
	}, tokens, "only the user's own keys, and only those with activity")

	for _, end := range ends {
		end()
	}
	ends = nil
	assert.Equal(t, service.RelayActivityCounts{Users: map[int]int64{}, Tokens: map[int]int64{}}, service.LocalRelayActivity())
}

func TestConsumeLogsAddCachedAndInputTokensToDashboardData(t *testing.T) {
	db := setupUsageActivityTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "alice", Password: "password123", AffCode: "alice-aff"}).Error)
	model.CacheQuotaDataLock.Lock()
	model.CacheQuotaData = make(map[string]*model.QuotaData)
	model.CacheQuotaDataLock.Unlock()

	record := func(modelName string, promptTokens int, public map[string]any) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx.Set("username", "alice")
		other := model.NewLogOther()
		other.MergePublic(public)
		model.RecordConsumeLog(ctx, 1, model.RecordConsumeLogParams{
			ModelName: modelName, PromptTokens: promptTokens, CompletionTokens: 10, Quota: 100, Other: other,
		})
	}
	// OpenAI usage: prompt tokens already include the cached ones. The first
	// save inserts the hour's row, the second adds to it.
	record("gpt-4o", 1000, map[string]any{"cache_tokens": 800})
	model.SaveQuotaDataCache()
	record("gpt-4o", 500, map[string]any{"cache_tokens": 0})
	record("gpt-4o", 300, map[string]any{"cache_tokens": 200})
	// Anthropic usage: cache reads and writes come on top of prompt tokens.
	record("claude-sonnet", 200, map[string]any{"usage_semantic": "anthropic", "cache_tokens": 600, "cache_write_tokens": 200})
	record("claude-sonnet", 100, map[string]any{"claude": true, "cache_creation_tokens_5m": 30, "cache_creation_tokens_1h": 20})
	model.SaveQuotaDataCache()

	now := common.GetTimestamp()
	target := "/api/data/self?start_timestamp=" + strconv.FormatInt(now-3600, 10) + "&end_timestamp=" + strconv.FormatInt(now, 10)
	var rows []model.QuotaData
	require.NoError(t, common.Unmarshal(callUsageActivity(t, GetUserQuotaDates, 1, target), &rows))
	got := map[string][4]int{}
	for _, row := range rows {
		got[row.ModelName] = [4]int{row.Count, row.TokenUsed, row.CacheTokens, row.InputTokens}
	}
	assert.Equal(t, map[string][4]int{
		"gpt-4o":        {3, 1830, 1000, 1800},
		"claude-sonnet": {2, 320, 600, 1150},
	}, got, "count, tokens, cached tokens, input tokens")
}
