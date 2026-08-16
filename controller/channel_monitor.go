package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type channelMonitorAdminDTO struct {
	ID                    int      `json:"id"`
	InternalName          string   `json:"internal_name"`
	PublicName            string   `json:"public_name"`
	Provider              string   `json:"provider"`
	APIMode               string   `json:"api_mode"`
	Endpoint              string   `json:"endpoint"`
	CredentialConfigured  bool     `json:"credential_configured"`
	CredentialMasked      string   `json:"credential_masked"`
	PrimaryModel          string   `json:"primary_model"`
	ExtraModels           []string `json:"extra_models"`
	IntervalSeconds       int      `json:"interval_seconds"`
	DegradedThresholdMs   int      `json:"degraded_threshold_ms"`
	TimeoutSeconds        int      `json:"timeout_seconds"`
	Enabled               bool     `json:"enabled"`
	PublicEnabled         bool     `json:"public_enabled"`
	LastCheckedAt         int64    `json:"last_checked_at"`
	NextCheckAt           int64    `json:"next_check_at"`
	ScheduleBlockedReason string   `json:"schedule_blocked_reason"`
	LatestStatus          string   `json:"latest_status"`
	LatestLatencyMs       *int64   `json:"latest_latency_ms"`
	LatestErrorCategory   string   `json:"latest_error_category"`
	LeaseOwner            string   `json:"lease_owner"`
	LeaseUntil            int64    `json:"lease_until"`
	CreatedBy             int      `json:"created_by"`
	CreatedAt             int64    `json:"created_at"`
	UpdatedAt             int64    `json:"updated_at"`
}

func ListChannelMonitors(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	enabled, err := parseChannelMonitorBoolQuery(c, "enabled")
	if err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}
	publicEnabled, err := parseChannelMonitorBoolQuery(c, "public_enabled")
	if err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}

	monitors, total, err := model.ListChannelMonitors(model.ChannelMonitorListFilter{
		Page:          pageInfo.Page,
		PageSize:      pageInfo.PageSize,
		Search:        c.Query("search"),
		Provider:      strings.ToLower(strings.TrimSpace(c.Query("provider"))),
		Enabled:       enabled,
		PublicEnabled: publicEnabled,
	})
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	latestChecks, err := model.GetLatestPrimaryChannelMonitorChecks(monitors)
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}

	items := make([]channelMonitorAdminDTO, 0, len(monitors))
	for _, monitor := range monitors {
		latest, exists := latestChecks[monitor.ID]
		var latestCheck *model.ChannelMonitorCheck
		if exists {
			latestCheck = &latest
		}
		item, err := channelMonitorToAdminDTO(monitor, latestCheck)
		if err != nil {
			channelMonitorAPIError(c, http.StatusInternalServerError, err)
			return
		}
		items = append(items, item)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateChannelMonitor(c *gin.Context) {
	input := service.ChannelMonitorInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}
	monitor, err := service.NormalizeChannelMonitorInput(input, nil, common.GetTimestamp())
	if err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}
	monitor.CreatedBy = c.GetInt("id")
	if err := model.CreateChannelMonitor(monitor); err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	service.InvalidateChannelMonitorPublicSummaryCache()
	item, err := channelMonitorToAdminDTO(monitor, nil)
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetChannelMonitor(c *gin.Context) {
	monitor, ok := requireChannelMonitor(c)
	if !ok {
		return
	}
	latestChecks, err := model.GetLatestPrimaryChannelMonitorChecks([]*model.ChannelMonitor{monitor})
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	latest, exists := latestChecks[monitor.ID]
	var latestCheck *model.ChannelMonitorCheck
	if exists {
		latestCheck = &latest
	}
	item, err := channelMonitorToAdminDTO(monitor, latestCheck)
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpdateChannelMonitor(c *gin.Context) {
	current, ok := requireChannelMonitor(c)
	if !ok {
		return
	}
	input := service.ChannelMonitorInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}
	monitor, err := service.NormalizeChannelMonitorInput(input, current, common.GetTimestamp())
	if err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}
	updateSchedule := !monitor.Enabled || strings.TrimSpace(input.APIKey) != "" || (!current.Enabled && monitor.Enabled)
	if err := model.UpdateChannelMonitor(monitor, updateSchedule); err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	service.InvalidateChannelMonitorPublicSummaryCache()
	item, err := channelMonitorToAdminDTO(monitor, nil)
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DeleteChannelMonitor(c *gin.Context) {
	monitor, ok := requireChannelMonitor(c)
	if !ok {
		return
	}
	if err := model.DeleteChannelMonitor(monitor.ID); err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	service.InvalidateChannelMonitorPublicSummaryCache()
	common.ApiSuccess(c, nil)
}

func GetChannelMonitorHistory(c *gin.Context) {
	monitor, ok := requireChannelMonitor(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	startTime, err := parseChannelMonitorInt64Query(c, "start_time")
	if err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}
	endTime, err := parseChannelMonitorInt64Query(c, "end_time")
	if err != nil {
		channelMonitorAPIError(c, http.StatusBadRequest, err)
		return
	}
	if startTime > 0 && endTime > 0 && startTime > endTime {
		channelMonitorAPIError(c, http.StatusBadRequest, errors.New("start_time must not exceed end_time"))
		return
	}
	items, total, err := model.ListChannelMonitorHistory(model.ChannelMonitorHistoryFilter{
		MonitorID:     monitor.ID,
		Page:          pageInfo.Page,
		PageSize:      pageInfo.PageSize,
		Model:         c.Query("model"),
		Status:        c.Query("status"),
		ErrorCategory: c.Query("error_category"),
		StartTime:     startTime,
		EndTime:       endTime,
	})
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetChannelMonitorPublicSummary(c *gin.Context) {
	summary, err := service.GetChannelMonitorPublicSummary()
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetChannelMonitorStatus(c *gin.Context) {
	days := 1
	if raw := strings.TrimSpace(c.Query("days")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			channelMonitorAPIError(c, http.StatusBadRequest, errors.New("invalid channel monitor availability days"))
			return
		}
		days = value
	}
	if days != 1 && days != 3 && days != 7 {
		channelMonitorAPIError(c, http.StatusBadRequest, errors.New("channel monitor availability days must be 1, 3, or 7"))
		return
	}
	status, err := service.GetPublishedChannelMonitorStatus(days)
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, status)
}

func GetChannelMonitorStatusDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		channelMonitorAPIError(c, http.StatusBadRequest, errors.New("invalid channel monitor ID"))
		return
	}
	detail, err := service.GetPublishedChannelMonitorDetail(id)
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	if detail == nil {
		channelMonitorAPIError(c, http.StatusNotFound, errors.New("channel monitor not found"))
		return
	}
	common.ApiSuccess(c, detail)
}

func RunChannelMonitorNow(c *gin.Context) {
	monitor, ok := requireChannelMonitor(c)
	if !ok {
		return
	}
	checks, err := service.RunChannelMonitorNow(c.Request.Context(), monitor.ID)
	if errors.Is(err, service.ErrChannelMonitorBusy) {
		channelMonitorAPIError(c, http.StatusConflict, err)
		return
	}
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, checks)
}

func requireChannelMonitor(c *gin.Context) (*model.ChannelMonitor, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		channelMonitorAPIError(c, http.StatusBadRequest, errors.New("invalid channel monitor ID"))
		return nil, false
	}
	monitor, err := model.GetChannelMonitor(id)
	if err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return nil, false
	}
	if monitor == nil {
		channelMonitorAPIError(c, http.StatusNotFound, errors.New("channel monitor not found"))
		return nil, false
	}
	return monitor, true
}

func channelMonitorToAdminDTO(monitor *model.ChannelMonitor, latestCheck *model.ChannelMonitorCheck) (channelMonitorAdminDTO, error) {
	if monitor == nil {
		return channelMonitorAdminDTO{}, errors.New("channel monitor is required")
	}
	extraModels := make([]string, 0)
	if monitor.ExtraModels != "" {
		if err := common.UnmarshalJsonStr(monitor.ExtraModels, &extraModels); err != nil {
			return channelMonitorAdminDTO{}, err
		}
	}
	latestStatus := model.ChannelMonitorStatusUnknown
	var latestLatencyMs *int64
	latestErrorCategory := ""
	if latestCheck != nil {
		latestStatus = latestCheck.Status
		latency := latestCheck.LatencyMs
		latestLatencyMs = &latency
		latestErrorCategory = latestCheck.ErrorCategory
	}
	return channelMonitorAdminDTO{
		ID:                    monitor.ID,
		InternalName:          monitor.InternalName,
		PublicName:            monitor.PublicName,
		Provider:              monitor.Provider,
		APIMode:               monitor.APIMode,
		Endpoint:              monitor.Endpoint,
		CredentialConfigured:  monitor.APIKeyEncrypted != "",
		CredentialMasked:      service.MaskChannelMonitorCredentialSuffix(monitor.APIKeySuffix),
		PrimaryModel:          monitor.PrimaryModel,
		ExtraModels:           extraModels,
		IntervalSeconds:       monitor.IntervalSeconds,
		DegradedThresholdMs:   monitor.DegradedThresholdMs,
		TimeoutSeconds:        monitor.TimeoutSeconds,
		Enabled:               monitor.Enabled,
		PublicEnabled:         monitor.PublicEnabled,
		LastCheckedAt:         monitor.LastCheckedAt,
		NextCheckAt:           monitor.NextCheckAt,
		ScheduleBlockedReason: monitor.ScheduleBlockedReason,
		LatestStatus:          latestStatus,
		LatestLatencyMs:       latestLatencyMs,
		LatestErrorCategory:   latestErrorCategory,
		LeaseOwner:            monitor.LeaseOwner,
		LeaseUntil:            monitor.LeaseUntil,
		CreatedBy:             monitor.CreatedBy,
		CreatedAt:             monitor.CreatedAt,
		UpdatedAt:             monitor.UpdatedAt,
	}, nil
}

func parseChannelMonitorBoolQuery(c *gin.Context, key string) (*bool, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, errors.New("invalid " + key + " filter")
	}
	return &value, nil
}

func parseChannelMonitorInt64Query(c *gin.Context, key string) (int64, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("invalid " + key + " filter")
	}
	return value, nil
}

func channelMonitorAPIError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{
		"success": false,
		"message": err.Error(),
	})
}
