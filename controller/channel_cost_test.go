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
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.ChannelCostHourly{}, &model.ChannelUpstreamBilling{}, &model.Option{}, &model.AuditLog{}))
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
		// Channel 9 was deleted; its usage is left out, so "legacy" is not listed.
		{BucketStart: today, Group: "legacy", ChannelId: 9, Quota: 7, BaseQuota: 7},
		{BucketStart: today, Group: "vip", ChannelId: 9, Quota: 70, BaseQuota: 70},
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

// upstreamCostTestChannel creates channel 7 pointing at baseURL with key.
func upstreamCostTestChannel(t *testing.T, db *gorm.DB, baseURL, key string) {
	t.Helper()
	require.NoError(t, db.Create(&model.Channel{Id: 7, Name: "upstream", Type: 1, Key: key, BaseURL: &baseURL, Group: "default", Status: 1}).Error)
}

type upstreamCostTestItem struct {
	Error *common.Message `json:"error"`
	Today struct {
		Usd   float64 `json:"usd"`
		Quota float64 `json:"quota"`
	} `json:"today"`
	Yesterday struct {
		Usd float64 `json:"usd"`
	} `json:"yesterday"`
	Week struct {
		Usd float64 `json:"usd"`
	} `json:"week"`
}

type upstreamAccountTestItem struct {
	Kind       string   `json:"kind"`
	ChannelIds []int    `json:"channel_ids"`
	Balance    float64  `json:"balance"`
	DailySpend float64  `json:"daily_spend"`
	DaysLeft   *float64 `json:"days_left"`
	Low        bool     `json:"low"`
}

var lastUpstreamAccounts []upstreamAccountTestItem

func getUpstreamCosts(t *testing.T, todayStart int64) upstreamCostTestItem {
	t.Helper()
	target := "/?refresh=true&timezone=Asia/Shanghai&today_start=" + strconv.FormatInt(todayStart, 10) +
		"&yesterday_start=" + strconv.FormatInt(todayStart-86400, 10) +
		"&week_start=" + strconv.FormatInt(todayStart-6*86400, 10)
	response := callChannelCost(t, GetChannelUpstreamCosts, http.MethodGet, target, nil, "")
	require.True(t, response.Success)
	var data struct {
		Channels []upstreamCostTestItem    `json:"channels"`
		Accounts []upstreamAccountTestItem `json:"accounts"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Len(t, data.Channels, 1)
	lastUpstreamAccounts = data.Accounts
	return data.Channels[0]
}

func TestChannelUpstreamBillingReadsSub2APISpend(t *testing.T) {
	db := setupChannelCostTestDB(t)
	reportsDaily := true
	var timezones []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/usage" || r.Header.Get("Authorization") != "Bearer sk-good" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
			return
		}
		timezones = append(timezones, r.URL.Query().Get("timezone"))
		if !reportsDaily {
			_, _ = w.Write([]byte(`{"mode":"unrestricted","balance":10}`))
			return
		}
		// Windows start at the viewer's midnight: today 2.5, yesterday 1.5.
		days := map[string]string{
			"1": `[{"actual_cost":2.5}]`,
			"2": `[{"actual_cost":1.5},{"actual_cost":2.5}]`,
			"7": `[{"actual_cost":6},{"actual_cost":4}]`,
		}[r.URL.Query().Get("days")]
		_, _ = w.Write([]byte(`{"mode":"unrestricted","remaining":10,"daily_usage":` + days + `}`))
	}))
	defer upstream.Close()
	upstreamCostTestChannel(t, db, upstream.URL, "sk-good")
	idParams := gin.Params{{Key: "id", Value: "7"}}

	reportsDaily = false
	old := callChannelCost(t, UpdateChannelUpstreamBilling, http.MethodPut, "/", idParams, `{"kind":"sub2api","usd_rate":0.5}`)
	assert.Equal(t, "This sub2api does not report daily usage. Update it to a newer version.", old.MessageKey)
	reportsDaily = true
	assert.Equal(t, "The dollar rate must be greater than 0 and at most {{max}}",
		callChannelCost(t, UpdateChannelUpstreamBilling, http.MethodPut, "/", idParams, `{"kind":"sub2api","usd_rate":0}`).MessageKey)
	saved := callChannelCost(t, UpdateChannelUpstreamBilling, http.MethodPut, "/", idParams, `{"kind":"sub2api","usd_rate":0.5}`)
	require.True(t, saved.Success, saved.MessageKey)

	todayStart := common.GetTimestamp() - 3600
	item := getUpstreamCosts(t, todayStart)
	require.Nil(t, item.Error)
	assert.InDelta(t, 2.5, item.Today.Usd, 1e-9)
	assert.InDelta(t, 2.5*0.5*common.QuotaPerUnit, item.Today.Quota, 1e-6)
	assert.InDelta(t, 1.5, item.Yesterday.Usd, 1e-9)
	assert.InDelta(t, 10, item.Week.Usd, 1e-9)
	assert.Contains(t, timezones, "Asia/Shanghai")
	// 4 dollars over the 25 hours since yesterday's start, against 10 left.
	require.Len(t, lastUpstreamAccounts, 1)
	account := lastUpstreamAccounts[0]
	assert.Equal(t, []int{7}, account.ChannelIds)
	assert.InDelta(t, 10, account.Balance, 1e-9)
	assert.InDelta(t, 4*24.0/25, account.DailySpend, 0.01)
	require.NotNil(t, account.DaysLeft)
	assert.InDelta(t, 10/(4*24.0/25), *account.DaysLeft, 0.01)
	assert.False(t, account.Low)

	require.NoError(t, db.Model(&model.Channel{}).Where("id = 7").Update("key", "sk-revoked").Error)
	failed := getUpstreamCosts(t, todayStart)
	require.NotNil(t, failed.Error)
	assert.Equal(t, "Upstream answered {{status}}: {{error}}", failed.Error.Key)
	assert.Equal(t, "Invalid API key", failed.Error.Params["error"])

	// A deleted channel is left out of the upstream bills too.
	require.NoError(t, db.Delete(&model.Channel{}, 7).Error)
	report := callChannelCost(t, GetChannelUpstreamCosts, http.MethodGet, "/?refresh=true&timezone=Asia/Shanghai&today_start="+strconv.FormatInt(todayStart, 10)+
		"&yesterday_start="+strconv.FormatInt(todayStart-86400, 10)+"&week_start="+strconv.FormatInt(todayStart-6*86400, 10), nil, "")
	require.True(t, report.Success)
	assert.Contains(t, string(report.Data), `"channels":[]`)

	cleared := callChannelCost(t, UpdateChannelUpstreamBilling, http.MethodPut, "/", idParams, `{"kind":""}`)
	require.True(t, cleared.Success)
	billings, err := model.GetChannelUpstreamBillings()
	require.NoError(t, err)
	assert.NotContains(t, billings, 7)
}

func TestChannelUpstreamBillingReadsNewAPIAccountStats(t *testing.T) {
	db := setupChannelCostTestDB(t)
	todayStart := common.GetTimestamp() - 3600
	tokenLookups := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":1000}}`))
		case "/api/usage/token/":
			tokenLookups++
			if r.Header.Get("Authorization") != "Bearer sk-good" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"code":true,"data":{"name":"cost key"}}`))
		case "/api/user/self":
			if r.Header.Get("Authorization") != "Bearer account-token" || r.Header.Get("New-Api-User") != "42" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":3000}}`))
		case "/api/log/self/stat":
			if r.Header.Get("Authorization") != "Bearer account-token" || r.Header.Get("New-Api-User") != "42" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"success":false,"message":"access token is invalid"}`))
				return
			}
			quota := 0
			if query.Get("token_name") == "cost key" {
				start, _ := strconv.ParseInt(query.Get("start_timestamp"), 10, 64)
				end, _ := strconv.ParseInt(query.Get("end_timestamp"), 10, 64)
				switch {
				case start == todayStart:
					quota = 3000
				case start == todayStart-86400 && end == todayStart-1:
					quota = 1000
				case start == todayStart-6*86400:
					quota = 9000
				}
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":` + strconv.Itoa(quota) + `}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	upstreamCostTestChannel(t, db, upstream.URL, "sk-good")
	idParams := gin.Params{{Key: "id", Value: "7"}}
	save := func(body string) channelCostResponse {
		return callChannelCost(t, UpdateChannelUpstreamBilling, http.MethodPut, "/", idParams, body)
	}

	assert.Equal(t, "Enter your user ID on the upstream", save(`{"kind":"newapi","usd_rate":1}`).MessageKey)
	assert.Equal(t, "Enter the system access token of your upstream account", save(`{"kind":"newapi","usd_rate":1,"upstream_user_id":42}`).MessageKey)
	assert.Equal(t, "Upstream answered {{status}}: {{error}}", save(`{"kind":"newapi","usd_rate":1,"upstream_user_id":42,"access_token":"wrong"}`).MessageKey)
	saved := save(`{"kind":"newapi","usd_rate":2,"upstream_user_id":42,"access_token":"account-token"}`)
	require.True(t, saved.Success, saved.MessageKey)
	kept := save(`{"kind":"newapi","usd_rate":2,"upstream_user_id":42}`)
	require.True(t, kept.Success, "an empty token keeps the saved one")

	target := "/?today_start=" + strconv.FormatInt(todayStart, 10) + "&yesterday_start=" + strconv.FormatInt(todayStart-86400, 10) +
		"&week_start=" + strconv.FormatInt(todayStart-6*86400, 10)
	stats := callChannelCost(t, GetChannelCostStats, http.MethodGet, target, nil, "")
	require.True(t, stats.Success)
	assert.Contains(t, string(stats.Data), `"upstream_billing":{"kind":"newapi","usd_rate":2},"newapi_login":{"upstream_user_id":42,"shared_with":[]}`)
	assert.NotContains(t, string(stats.Data), "account-token")

	item := getUpstreamCosts(t, todayStart)
	require.Nil(t, item.Error)
	assert.InDelta(t, 3, item.Today.Usd, 1e-9)
	assert.InDelta(t, 3*2*common.QuotaPerUnit, item.Today.Quota, 1e-6)
	assert.InDelta(t, 1, item.Yesterday.Usd, 1e-9)
	assert.InDelta(t, 9, item.Week.Usd, 1e-9)
	getUpstreamCosts(t, todayStart)
	// new-api allows 20 token lookups per 20 minutes per IP; names are kept.
	assert.Equal(t, 1, tokenLookups)
	// 3 dollars left at about 3.84 a day lasts less than a day.
	require.Len(t, lastUpstreamAccounts, 1)
	assert.InDelta(t, 3, lastUpstreamAccounts[0].Balance, 1e-9)
	require.NotNil(t, lastUpstreamAccounts[0].DaysLeft)
	assert.Less(t, *lastUpstreamAccounts[0].DaysLeft, 1.0)
	assert.True(t, lastUpstreamAccounts[0].Low)

	// The token is only sent to the address it was entered for.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = 7").Update("base_url", "https://elsewhere.example").Error)
	moved := getUpstreamCosts(t, todayStart)
	require.NotNil(t, moved.Error)
	assert.Equal(t, "The channel address changed. Enter the access token again.", moved.Error.Key)
	assert.Equal(t, "Enter the system access token of your upstream account", save(`{"kind":"newapi","usd_rate":2,"upstream_user_id":42}`).MessageKey)
}

func TestChannelUpstreamBillingSharesNewAPILoginByAddress(t *testing.T) {
	db := setupChannelCostTestDB(t)
	validToken := "account-token"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":1000}}`))
		case "/api/usage/token/":
			name := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer sk-")
			_, _ = w.Write([]byte(`{"code":true,"data":{"name":"key ` + name + `"}}`))
		case "/api/user/self", "/api/log/self/stat":
			if r.Header.Get("Authorization") != "Bearer "+validToken || r.Header.Get("New-Api-User") != "42" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"success":false,"message":"access token is invalid"}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":1000}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	sameWithSlash := upstream.URL + "/"
	elsewhere := "https://elsewhere.example"
	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 7, Name: "relay-a", Type: 1, Key: "sk-a", BaseURL: &upstream.URL, Group: "default", Status: 1},
		{Id: 8, Name: "relay-b", Type: 1, Key: "sk-b", BaseURL: &sameWithSlash, Group: "default", Status: 1},
		{Id: 9, Name: "other", Type: 1, Key: "sk-c", BaseURL: &elsewhere, Group: "default", Status: 1},
	}).Error)
	save := func(id, body string) channelCostResponse {
		return callChannelCost(t, UpdateChannelUpstreamBilling, http.MethodPut, "/", gin.Params{{Key: "id", Value: id}}, body)
	}
	todayStart := common.GetTimestamp() - 3600
	logins := func() map[int]*channelNewAPILoginView {
		target := "/?today_start=" + strconv.FormatInt(todayStart, 10) + "&yesterday_start=" + strconv.FormatInt(todayStart-86400, 10) +
			"&week_start=" + strconv.FormatInt(todayStart-6*86400, 10)
		response := callChannelCost(t, GetChannelCostStats, http.MethodGet, target, nil, "")
		require.True(t, response.Success)
		var stats channelCostStats
		require.NoError(t, common.Unmarshal(response.Data, &stats))
		byChannel := map[int]*channelNewAPILoginView{}
		for _, channel := range stats.Groups[0].Channels {
			byChannel[channel.Id] = channel.NewAPILogin
		}
		return byChannel
	}

	saved := save("7", `{"kind":"newapi","usd_rate":1,"upstream_user_id":42,"access_token":"account-token"}`)
	require.True(t, saved.Success, saved.MessageKey)
	offered := logins()
	assert.Equal(t, &channelNewAPILoginView{UpstreamUserId: 42, SharedWith: []channelCostChannelRef{{Id: 7, Name: "relay-a"}}}, offered[8],
		"a channel on the same address is offered the saved login")
	assert.Nil(t, offered[9], "a channel on another address is not")

	reused := save("8", `{"kind":"newapi","usd_rate":1,"upstream_user_id":42}`)
	require.True(t, reused.Success, reused.MessageKey)
	assert.Equal(t, "Enter the system access token of your upstream account",
		save("9", `{"kind":"newapi","usd_rate":1,"upstream_user_id":42}`).MessageKey, "a token is never reused on another address")

	validToken = "rotated-token"
	rotated := save("8", `{"kind":"newapi","usd_rate":1,"upstream_user_id":42,"access_token":"rotated-token"}`)
	require.True(t, rotated.Success, rotated.MessageKey)
	assert.Equal(t, []channelCostChannelRef{{Id: 8, Name: "relay-b"}}, logins()[7].SharedWith)

	target := "/?refresh=true&timezone=Asia/Shanghai&today_start=" + strconv.FormatInt(todayStart, 10) +
		"&yesterday_start=" + strconv.FormatInt(todayStart-86400, 10) + "&week_start=" + strconv.FormatInt(todayStart-6*86400, 10)
	response := callChannelCost(t, GetChannelUpstreamCosts, http.MethodGet, target, nil, "")
	require.True(t, response.Success)
	var report struct {
		Channels []upstreamCostTestItem    `json:"channels"`
		Accounts []upstreamAccountTestItem `json:"accounts"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &report))
	require.Len(t, report.Channels, 2)
	for _, item := range report.Channels {
		assert.Nil(t, item.Error, "the rotated token reaches both channels")
	}
	require.Len(t, report.Accounts, 1)
	assert.Equal(t, []int{7, 8}, report.Accounts[0].ChannelIds)
}
