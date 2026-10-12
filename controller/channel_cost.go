package controller

import (
	"errors"
	"slices"
	"strconv"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	channelCostDefaultRatioOption  = "channel_cost_setting.default_ratio"
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
	// Ratio is the channel's own ratio; nil means it uses the default.
	Ratio          *float64 `json:"ratio"`
	TodayQuota     int64    `json:"today_quota"`
	YesterdayQuota int64    `json:"yesterday_quota"`
	WeekQuota      int64    `json:"week_quota"`
}

// GetChannelCostStats returns every channel's net usage today, yesterday and in
// the last 7 days. The client sends the range starts in its own time zone.
func GetChannelCostStats(c *gin.Context) {
	todayStart, errToday := strconv.ParseInt(c.Query("today_start"), 10, 64)
	yesterdayStart, errYesterday := strconv.ParseInt(c.Query("yesterday_start"), 10, 64)
	weekStart, errWeek := strconv.ParseInt(c.Query("week_start"), 10, 64)
	now := common.GetTimestamp()
	if errToday != nil || errYesterday != nil || errWeek != nil ||
		weekStart > yesterdayStart || yesterdayStart > todayStart || todayStart > now ||
		now-weekStart > channelCostMaxRangeSeconds {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	totals, err := model.GetChannelCostTotals(weekStart, yesterdayStart, todayStart)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var channels []model.Channel
	if err := model.DB.Select("id", "name", "type", "status").Order("id").Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	setting := operation_setting.GetChannelCostSetting()
	items := make([]channelCostChannel, 0, len(channels))
	listed := make(map[int]bool, len(channels))
	for _, channel := range channels {
		listed[channel.Id] = true
		items = append(items, channelCostChannel{Id: channel.Id, Name: channel.Name, Type: channel.Type, Status: channel.Status})
	}
	// Usage of channels that were deleted since still counts toward totals.
	for channelId := range totals {
		if !listed[channelId] {
			items = append(items, channelCostChannel{Id: channelId, Deleted: true})
		}
	}
	for i := range items {
		if ratio, ok := setting.ChannelRatios[items[i].Id]; ok {
			items[i].Ratio = &ratio
		}
		if total := totals[items[i].Id]; total != nil {
			items[i].TodayQuota = total.Today
			items[i].YesterdayQuota = total.Yesterday
			items[i].WeekQuota = total.Week
		}
	}
	slices.SortStableFunc(items, func(a, b channelCostChannel) int {
		if a.WeekQuota != b.WeekQuota {
			if a.WeekQuota > b.WeekQuota {
				return -1
			}
			return 1
		}
		return a.Id - b.Id
	})

	common.ApiSuccess(c, gin.H{
		"default_ratio":       setting.DefaultRatio,
		"log_consume_enabled": common.LogConsumeEnabled,
		"channels":            items,
	})
}

type channelCostRatioRequest struct {
	Ratio *float64 `json:"ratio"`
}

func UpdateChannelCostDefaultRatio(c *gin.Context) {
	var request channelCostRatioRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Ratio == nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if !operation_setting.IsValidChannelCostRatio(*request.Ratio) {
		common.ApiErrorT(c, "Cost ratio must be between 0 and {{max}}", map[string]any{"max": operation_setting.MaxChannelCostRatio})
		return
	}
	channelCostSettingMu.Lock()
	defer channelCostSettingMu.Unlock()
	err := model.UpdateOptionsBulk(map[string]string{
		channelCostDefaultRatioOption: strconv.FormatFloat(*request.Ratio, 'f', -1, 64),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update", map[string]any{"key": channelCostDefaultRatioOption})
	common.ApiSuccess(c, nil)
}

// UpdateChannelCostRatio sets a channel's own ratio, or clears it when ratio
// is null so the channel falls back to the default.
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
