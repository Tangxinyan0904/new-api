package controller

import (
	"cmp"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	channelCostChannelRatiosOption = "channel_cost_setting.channel_ratios"
	// channelCostMaxRangeSeconds is how far back the page may ask; the rollup
	// task keeps at least this much.
	channelCostMaxRangeSeconds = 9 * 24 * 3600
)

// channelCostSettingMu serializes read-modify-write updates of the ratios.
var channelCostSettingMu sync.Mutex

type channelCostChannel struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Type    int    `json:"type"`
	Status  int    `json:"status"`
	Deleted bool   `json:"deleted"`
	// Ratio is the channel's cost ratio, shared by all its groups; nil means
	// none was entered.
	Ratio *float64 `json:"ratio"`
	// UpstreamBilling is set when the cost is read from the upstream's bill.
	UpstreamBilling *channelUpstreamBillingView `json:"upstream_billing"`
	Today           model.ChannelCostUsage      `json:"today"`
	Yesterday       model.ChannelCostUsage      `json:"yesterday"`
	Week            model.ChannelCostUsage      `json:"week"`
}

// channelUpstreamBillingView is a channel's upstream billing without the
// access token.
type channelUpstreamBillingView struct {
	Kind           string  `json:"kind"`
	UsdRate        float64 `json:"usd_rate"`
	UpstreamUserId int     `json:"upstream_user_id"`
	AccessTokenSet bool    `json:"access_token_set"`
}

type channelCostGroup struct {
	Name string `json:"name"`
	// Ratio is the group's configured ratio; nil when it has none.
	Ratio    *float64             `json:"ratio"`
	Channels []channelCostChannel `json:"channels"`
}

// channelCostTimezonePattern accepts IANA time zone names such as
// Asia/Shanghai or Etc/GMT-8.
var channelCostTimezonePattern = regexp.MustCompile(`^[A-Za-z0-9_+/-]{1,64}$`)

// channelCostRanges reads the range starts the client sent in its own time
// zone, and the time zone name when there is one.
func channelCostRanges(c *gin.Context) (service.UpstreamCostRanges, bool) {
	todayStart, errToday := strconv.ParseInt(c.Query("today_start"), 10, 64)
	yesterdayStart, errYesterday := strconv.ParseInt(c.Query("yesterday_start"), 10, 64)
	weekStart, errWeek := strconv.ParseInt(c.Query("week_start"), 10, 64)
	timezone := c.Query("timezone")
	now := common.GetTimestamp()
	if errToday != nil || errYesterday != nil || errWeek != nil ||
		weekStart > yesterdayStart || yesterdayStart > todayStart || todayStart > now ||
		now-weekStart > channelCostMaxRangeSeconds ||
		(timezone != "" && !channelCostTimezonePattern.MatchString(timezone)) {
		common.ApiErrorT(c, "Invalid parameters")
		return service.UpstreamCostRanges{}, false
	}
	return service.UpstreamCostRanges{
		TodayStart:     todayStart,
		YesterdayStart: yesterdayStart,
		WeekStart:      weekStart,
		Now:            now,
		Timezone:       timezone,
	}, true
}

// GetChannelCostStats returns the net usage of every channel in each of its
// groups today, yesterday and in the last 7 days. The client sends the range
// starts in its own time zone.
func GetChannelCostStats(c *gin.Context) {
	ranges, ok := channelCostRanges(c)
	if !ok {
		return
	}
	totals, err := model.GetChannelCostTotals(ranges.WeekStart, ranges.YesterdayStart, ranges.TodayStart)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var channels []model.Channel
	if err := model.DB.Select("id", "name", "type", "status", "group").Order("id").Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	channelRatios := operation_setting.GetChannelCostSetting().ChannelRatios
	billings, err := model.GetChannelUpstreamBillings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channelsById := make(map[int]model.Channel, len(channels))
	members := make(map[model.ChannelCostKey]bool)
	for _, channel := range channels {
		channelsById[channel.Id] = channel
		for _, group := range channel.GetGroups() {
			if group != "" {
				members[model.ChannelCostKey{Group: group, ChannelId: channel.Id}] = true
			}
		}
	}
	// Usage counts even if the channel has left the group or was deleted.
	for key := range totals {
		members[key] = true
	}

	groupRatios := ratio_setting.GetGroupRatioCopy()
	groupsByName := make(map[string]*channelCostGroup)
	for key := range members {
		group := groupsByName[key.Group]
		if group == nil {
			group = &channelCostGroup{Name: key.Group, Channels: []channelCostChannel{}}
			if ratio, ok := groupRatios[key.Group]; ok {
				group.Ratio = &ratio
			}
			groupsByName[key.Group] = group
		}
		item := channelCostChannel{Id: key.ChannelId, Deleted: true}
		if channel, ok := channelsById[key.ChannelId]; ok {
			item = channelCostChannel{Id: channel.Id, Name: channel.Name, Type: channel.Type, Status: channel.Status}
		}
		if ratio, ok := channelRatios[key.ChannelId]; ok {
			item.Ratio = &ratio
		}
		if billing, ok := billings[key.ChannelId]; ok {
			item.UpstreamBilling = &channelUpstreamBillingView{
				Kind:           billing.Kind,
				UsdRate:        billing.UsdRate,
				UpstreamUserId: billing.UpstreamUserId,
				AccessTokenSet: billing.AccessToken != "",
			}
		}
		if total := totals[key]; total != nil {
			item.Today, item.Yesterday, item.Week = total.Today, total.Yesterday, total.Week
		}
		group.Channels = append(group.Channels, item)
	}

	// Busiest first: groups and their channels by 7-day usage, then by name/id.
	groups := make([]channelCostGroup, 0, len(groupsByName))
	groupWeekQuota := make(map[string]int64, len(groupsByName))
	for _, group := range groupsByName {
		slices.SortFunc(group.Channels, func(a, b channelCostChannel) int {
			if a.Week.Quota != b.Week.Quota {
				return cmp.Compare(b.Week.Quota, a.Week.Quota)
			}
			return cmp.Compare(a.Id, b.Id)
		})
		for _, channel := range group.Channels {
			groupWeekQuota[group.Name] += channel.Week.Quota
		}
		groups = append(groups, *group)
	}
	slices.SortFunc(groups, func(a, b channelCostGroup) int {
		if groupWeekQuota[a.Name] != groupWeekQuota[b.Name] {
			return cmp.Compare(groupWeekQuota[b.Name], groupWeekQuota[a.Name])
		}
		return cmp.Compare(a.Name, b.Name)
	})

	common.ApiSuccess(c, gin.H{
		"log_consume_enabled": common.LogConsumeEnabled,
		"groups":              groups,
	})
}

type channelCostRatioRequest struct {
	Ratio *float64 `json:"ratio"`
}

// UpdateChannelCostRatio sets a channel's cost ratio, or clears it when ratio
// is null so the channel's usage counts as cost.
func UpdateChannelCostRatio(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	var request channelCostRatioRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if request.Ratio != nil && !operation_setting.IsValidChannelCostRatio(*request.Ratio) {
		common.ApiErrorT(c, "Cost ratio must be between 0 and {{max}}", map[string]any{"max": operation_setting.MaxChannelCostRatio})
		return
	}
	channelCostSettingMu.Lock()
	defer channelCostSettingMu.Unlock()
	ratios := operation_setting.GetChannelCostSetting().ChannelRatios
	if request.Ratio == nil {
		delete(ratios, channelId)
	} else {
		if _, err := model.GetChannelById(channelId, false); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				common.ApiErrorT(c, "Channel not found")
				return
			}
			common.ApiError(c, err)
			return
		}
		ratios[channelId] = *request.Ratio
	}
	encoded, err := common.Marshal(ratios)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOptionsBulk(map[string]string{channelCostChannelRatiosOption: string(encoded)}); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update", map[string]any{"key": channelCostChannelRatiosOption})
	common.ApiSuccess(c, nil)
}

// maxUpstreamUsdRate bounds how many of this site's dollars one upstream
// dollar can be set to.
const maxUpstreamUsdRate = 100

// channelUpstreamCostAmount is an upstream charge in the upstream's dollars
// and in this site's quota units.
type channelUpstreamCostAmount struct {
	Usd   float64 `json:"usd"`
	Quota float64 `json:"quota"`
}

type channelUpstreamCostItem struct {
	ChannelId int    `json:"channel_id"`
	Kind      string `json:"kind"`
	// Error says why the upstream could not be read; the amounts are then zero.
	Error     *common.Message           `json:"error"`
	FetchedAt int64                     `json:"fetched_at"`
	Today     channelUpstreamCostAmount `json:"today"`
	Yesterday channelUpstreamCostAmount `json:"yesterday"`
	Week      channelUpstreamCostAmount `json:"week"`
}

// GetChannelUpstreamCosts reads what each channel with upstream billing was
// charged by its upstream today, yesterday and in the last 7 days, and how
// long each upstream account's balance lasts. Answers are cached for five
// minutes unless refresh is set.
func GetChannelUpstreamCosts(c *gin.Context) {
	ranges, ok := channelCostRanges(c)
	if !ok {
		return
	}
	if ranges.Timezone == "" {
		ranges.Timezone = "UTC"
	}
	billings, err := model.GetChannelUpstreamBillings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ids := make([]int, 0, len(billings))
	for id := range billings {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var channels []*model.Channel
	if len(ids) > 0 {
		if err := model.DB.Where("id IN ?", ids).Find(&channels).Error; err != nil {
			common.ApiError(c, err)
			return
		}
	}
	results := service.QueryChannelUpstreamCosts(c.Request.Context(), channels, billings, ranges, c.Query("refresh") == "true")
	items := make([]channelUpstreamCostItem, 0, len(ids))
	for _, id := range ids {
		billing := billings[id]
		item := channelUpstreamCostItem{ChannelId: id, Kind: billing.Kind}
		result, found := results[id]
		if !found {
			item.Error = common.NewMessage("Channel not found")
			items = append(items, item)
			continue
		}
		item.FetchedAt = result.FetchedAt
		if result.Err != nil {
			var message *common.Message
			if !errors.As(result.Err, &message) {
				message = common.NewMessage("Upstream unreachable: {{error}}", map[string]any{"error": result.Err.Error()})
			}
			item.Error = message
			items = append(items, item)
			continue
		}
		toAmount := func(usd float64) channelUpstreamCostAmount {
			return channelUpstreamCostAmount{Usd: usd, Quota: usd * billing.UsdRate * common.QuotaPerUnit}
		}
		item.Today = toAmount(result.Cost.Today)
		item.Yesterday = toAmount(result.Cost.Yesterday)
		item.Week = toAmount(result.Cost.Week)
		items = append(items, item)
	}
	accounts := service.ForecastUpstreamAccounts(channels, billings, results, ranges)
	common.ApiSuccess(c, gin.H{"channels": items, "accounts": accounts})
}

type channelUpstreamBillingRequest struct {
	Kind           string  `json:"kind"`
	UsdRate        float64 `json:"usd_rate"`
	AccessToken    string  `json:"access_token"`
	UpstreamUserId int     `json:"upstream_user_id"`
}

// UpdateChannelUpstreamBilling sets where a channel's cost is read from. An
// empty kind goes back to the cost ratio. A new-api access token left empty
// keeps the saved one. The settings are checked against the upstream first.
func UpdateChannelUpstreamBilling(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	var request channelUpstreamBillingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if request.Kind == "" {
		if err := model.DeleteChannelUpstreamBilling(channelId); err != nil {
			common.ApiError(c, err)
			return
		}
		recordManageAudit(c, "channel_cost.upstream_billing", map[string]any{"id": channelId, "kind": "none"})
		common.ApiSuccess(c, nil)
		return
	}
	if request.Kind != model.UpstreamBillingSub2API && request.Kind != model.UpstreamBillingNewAPI {
		common.ApiErrorT(c, "Unsupported upstream type")
		return
	}
	if !(request.UsdRate > 0 && request.UsdRate <= maxUpstreamUsdRate) {
		common.ApiErrorT(c, "The dollar rate must be greater than 0 and at most {{max}}", map[string]any{"max": maxUpstreamUsdRate})
		return
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorT(c, "Channel not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	billing := model.ChannelUpstreamBilling{
		ChannelId: channelId,
		Kind:      request.Kind,
		UsdRate:   request.UsdRate,
		UpdatedAt: common.GetTimestamp(),
	}
	if request.Kind == model.UpstreamBillingNewAPI {
		if request.UpstreamUserId <= 0 {
			common.ApiErrorT(c, "Enter your user ID on the upstream")
			return
		}
		billing.UpstreamUserId = request.UpstreamUserId
		billing.BaseURL = channel.GetBaseURL()
		billing.AccessToken = strings.TrimSpace(request.AccessToken)
		if billing.AccessToken == "" {
			// Keep the saved token, but only for the address it was entered for.
			saved, err := model.GetChannelUpstreamBilling(channelId)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			if saved == nil || saved.AccessToken == "" || saved.BaseURL != billing.BaseURL {
				common.ApiErrorT(c, "Enter the system access token of your upstream account")
				return
			}
			billing.AccessToken = saved.AccessToken
		}
	}
	if err := service.CheckChannelUpstreamBilling(c.Request.Context(), channel, billing); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveChannelUpstreamBilling(&billing); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "channel_cost.upstream_billing", map[string]any{"id": channelId, "kind": billing.Kind})
	common.ApiSuccess(c, nil)
}
