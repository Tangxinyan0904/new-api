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

	items := make([]channelMonitorAdminDTO, 0, len(monitors))
	for _, monitor := range monitors {
		item, err := channelMonitorToAdminDTO(monitor)
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
	item, err := channelMonitorToAdminDTO(monitor)
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
	item, err := channelMonitorToAdminDTO(monitor)
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
	if err := model.UpdateChannelMonitor(monitor); err != nil {
		channelMonitorAPIError(c, http.StatusInternalServerError, err)
		return
	}
	item, err := channelMonitorToAdminDTO(monitor)
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
	common.ApiSuccess(c, nil)
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

func channelMonitorToAdminDTO(monitor *model.ChannelMonitor) (channelMonitorAdminDTO, error) {
	if monitor == nil {
		return channelMonitorAdminDTO{}, errors.New("channel monitor is required")
	}
	extraModels := make([]string, 0)
	if monitor.ExtraModels != "" {
		if err := common.UnmarshalJsonStr(monitor.ExtraModels, &extraModels); err != nil {
			return channelMonitorAdminDTO{}, err
		}
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

func channelMonitorAPIError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{
		"success": false,
		"message": err.Error(),
	})
}
