package controller

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	Ratio     *float64               `json:"ratio"`
	Today     model.ChannelCostUsage `json:"today"`
	Yesterday model.ChannelCostUsage `json:"yesterday"`
	Week      model.ChannelCostUsage `json:"week"`
}

type channelCostGroup struct {
	Name string `json:"name"`
	// Ratio is the group's configured ratio; nil when it has none.
	Ratio    *float64             `json:"ratio"`
	Channels []channelCostChannel `json:"channels"`
}

// GetChannelCostStats returns the net usage of every channel in each of its
// groups today, yesterday and in the last 7 days. The client sends the range
// starts in its own time zone.
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
	if err := model.DB.Select("id", "name", "type", "status", "group").Order("id").Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	channelRatios := operation_setting.GetChannelCostSetting().ChannelRatios
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
