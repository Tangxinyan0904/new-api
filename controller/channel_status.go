package controller

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const channelStatusPublicNameMaxRunes = 128

type channelStatusCounts struct {
	Operational int `json:"operational"`
	Degraded    int `json:"degraded"`
	Failed      int `json:"failed"`
	Unknown     int `json:"unknown"`
}

func (counts *channelStatusCounts) add(status string) {
	switch status {
	case service.ChannelStatusOperational:
		counts.Operational++
	case service.ChannelStatusDegraded:
		counts.Degraded++
	case service.ChannelStatusFailed:
		counts.Failed++
	default:
		counts.Unknown++
	}
}

type channelStatusTimelinePoint struct {
	StartAt int64 `json:"start_at"`
	service.ChannelStatusFigures
}

type channelStatusItem struct {
	Id         int    `json:"id"`
	PublicName string `json:"public_name"`
	NameColor  string `json:"name_color"`
	// Group is only sent to staff.
	Group         string `json:"group,omitempty"`
	PublicEnabled bool   `json:"public_enabled"`
	service.ChannelStatusFigures
	Timeline []channelStatusTimelinePoint `json:"timeline"`
}

type channelStatusModel struct {
	Model string `json:"model"`
	service.ChannelStatusFigures
}

type channelStatusResponse struct {
	LastUpdatedAt int64               `json:"last_updated_at"`
	WindowStart   int64               `json:"window_start"`
	BucketSeconds int                 `json:"bucket_seconds"`
	Counts        channelStatusCounts `json:"counts"`
	Items         []channelStatusItem `json:"items"`
}

func channelStatusMonitorIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, model.ErrChannelStatusMonitorNotFound)
		return 0, false
	}
	return id, true
}

// channelStatusStaff reports whether the caller may see private monitors and
// the groups behind them.
func channelStatusStaff(c *gin.Context) bool {
	return authz.Can(c.GetInt("id"), c.GetInt("role"), authz.ChannelRead)
}

func channelStatusItemOf(monitor model.ChannelStatusMonitor, snapshot *service.ChannelStatusSnapshot, staff bool) channelStatusItem {
	stats := snapshot.Groups[monitor.Group]
	if stats == nil {
		stats = &service.ChannelStatusGroupStats{}
	}
	item := channelStatusItem{
		Id:                   monitor.Id,
		PublicName:           monitor.PublicName,
		NameColor:            monitor.NameColor,
		PublicEnabled:        monitor.PublicEnabled,
		ChannelStatusFigures: service.ChannelStatusFiguresOf(stats.Total),
		Timeline:             make([]channelStatusTimelinePoint, 0, len(stats.Buckets)),
	}
	if staff {
		item.Group = monitor.Group
	}
	for bucket, tally := range stats.Buckets {
		item.Timeline = append(item.Timeline, channelStatusTimelinePoint{
			StartAt:              snapshot.WindowStart + int64(bucket)*service.ChannelStatusBucketSeconds,
			ChannelStatusFigures: service.ChannelStatusFiguresOf(tally),
		})
	}
	return item
}

// GetChannelStatus returns the last hour of every monitor the caller can see.
func GetChannelStatus(c *gin.Context) {
	staff := channelStatusStaff(c)
	monitors, err := model.ListChannelStatusMonitors(staff)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	snapshot, err := service.GetChannelStatusSnapshot()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response := channelStatusResponse{
		LastUpdatedAt: snapshot.UpdatedAt,
		WindowStart:   snapshot.WindowStart,
		BucketSeconds: service.ChannelStatusBucketSeconds,
		Items:         make([]channelStatusItem, 0, len(monitors)),
	}
	for _, monitor := range monitors {
		item := channelStatusItemOf(monitor, snapshot, staff)
		response.Counts.add(item.Status)
		response.Items = append(response.Items, item)
	}
	common.ApiSuccess(c, response)
}

// GetChannelStatusDetail returns one monitor with its models, busiest first.
func GetChannelStatusDetail(c *gin.Context) {
	id, ok := channelStatusMonitorIdParam(c)
	if !ok {
		return
	}
	staff := channelStatusStaff(c)
	monitor, err := model.GetChannelStatusMonitor(id, staff)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	snapshot, err := service.GetChannelStatusSnapshot()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	models := make([]channelStatusModel, 0)
	if stats := snapshot.Groups[monitor.Group]; stats != nil {
		for name, tally := range stats.Models {
			models = append(models, channelStatusModel{Model: name, ChannelStatusFigures: service.ChannelStatusFiguresOf(*tally)})
		}
	}
	slices.SortFunc(models, func(a, b channelStatusModel) int {
		return cmp.Or(
			cmp.Compare(b.SuccessCount+b.FailureCount, a.SuccessCount+a.FailureCount),
			cmp.Compare(a.Model, b.Model),
		)
	})
	common.ApiSuccess(c, gin.H{
		"last_updated_at": snapshot.UpdatedAt,
		"monitor":         channelStatusItemOf(*monitor, snapshot, staff),
		"models":          models,
	})
}

// GetChannelStatusPublicSummary returns the status of the public monitors for
// pages that need no sign-in, such as the model square.
func GetChannelStatusPublicSummary(c *gin.Context) {
	monitors, err := model.ListChannelStatusMonitors(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	snapshot, err := service.GetChannelStatusSnapshot()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	type publicMonitor struct {
		Id          int      `json:"id"`
		PublicName  string   `json:"public_name"`
		NameColor   string   `json:"name_color"`
		Status      string   `json:"status"`
		SuccessRate *float64 `json:"success_rate"`
	}
	counts := channelStatusCounts{}
	items := make([]publicMonitor, 0, len(monitors))
	for _, monitor := range monitors {
		item := channelStatusItemOf(monitor, snapshot, false)
		counts.add(item.Status)
		items = append(items, publicMonitor{
			Id:          item.Id,
			PublicName:  item.PublicName,
			NameColor:   item.NameColor,
			Status:      item.Status,
			SuccessRate: item.SuccessRate,
		})
	}
	common.ApiSuccess(c, gin.H{
		"last_updated_at": snapshot.UpdatedAt,
		"counts":          counts,
		"monitors":        items,
	})
}

// channelStatusMonitorFromRequest reads and validates a monitor from the body.
func channelStatusMonitorFromRequest(c *gin.Context) (*model.ChannelStatusMonitor, bool) {
	var request struct {
		Group         string `json:"group"`
		PublicName    string `json:"public_name"`
		NameColor     string `json:"name_color"`
		PublicEnabled bool   `json:"public_enabled"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return nil, false
	}
	monitor := &model.ChannelStatusMonitor{
		Group:         strings.TrimSpace(request.Group),
		PublicName:    strings.TrimSpace(request.PublicName),
		NameColor:     strings.ToUpper(strings.TrimSpace(request.NameColor)),
		PublicEnabled: request.PublicEnabled,
	}
	if monitor.Group == "" || !ratio_setting.ContainsGroupRatio(monitor.Group) {
		common.ApiErrorT(c, "This group does not exist")
		return nil, false
	}
	if monitor.PublicName == "" {
		common.ApiErrorT(c, "Display name is required")
		return nil, false
	}
	if utf8.RuneCountInString(monitor.PublicName) > channelStatusPublicNameMaxRunes {
		common.ApiErrorT(c, "Display name must be at most {{max}} characters", map[string]any{"max": channelStatusPublicNameMaxRunes})
		return nil, false
	}
	if monitor.NameColor != "" {
		valid := len(monitor.NameColor) == 7 && monitor.NameColor[0] == '#'
		for _, r := range monitor.NameColor[1:] {
			valid = valid && strings.ContainsRune("0123456789ABCDEF", r)
		}
		if !valid {
			common.ApiErrorT(c, "Name color must use #RRGGBB format")
			return nil, false
		}
	}
	return monitor, true
}

func CreateChannelStatusMonitor(c *gin.Context) {
	monitor, ok := channelStatusMonitorFromRequest(c)
	if !ok {
		return
	}
	monitor.CreatedBy = c.GetInt("id")
	if err := model.SaveChannelStatusMonitor(monitor); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateChannelStatusSnapshot()
	common.ApiSuccess(c, monitor)
}

func UpdateChannelStatusMonitor(c *gin.Context) {
	id, ok := channelStatusMonitorIdParam(c)
	if !ok {
		return
	}
	monitor, ok := channelStatusMonitorFromRequest(c)
	if !ok {
		return
	}
	monitor.Id = id
	if err := model.SaveChannelStatusMonitor(monitor); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateChannelStatusSnapshot()
	common.ApiSuccess(c, monitor)
}

func DeleteChannelStatusMonitor(c *gin.Context) {
	id, ok := channelStatusMonitorIdParam(c)
	if !ok {
		return
	}
	if err := model.DeleteChannelStatusMonitor(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateChannelStatusSnapshot()
	common.ApiSuccess(c, nil)
}
