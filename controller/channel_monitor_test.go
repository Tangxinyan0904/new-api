package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelMonitorAdminListDoesNotExposeCredentialCiphertext(t *testing.T) {
	setupChannelMonitorControllerTestDB(t)
	monitor := &model.ChannelMonitor{
		InternalName:        "internal monitor",
		PublicName:          "Public Monitor",
		Provider:            model.ChannelMonitorProviderOpenAI,
		APIMode:             model.ChannelMonitorAPIModeChatCompletions,
		Endpoint:            "https://example.com",
		APIKeyEncrypted:     "ciphertext-recognizable-marker",
		APIKeySuffix:        "last",
		PrimaryModel:        "gpt-test",
		ExtraModels:         `["gpt-test-mini"]`,
		IntervalSeconds:     60,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
		Enabled:             true,
		PublicEnabled:       true,
		NextCheckAt:         100,
	}
	require.NoError(t, model.CreateChannelMonitor(monitor))
	require.NoError(t, model.DB.Create(&model.ChannelMonitorCheck{
		MonitorID: monitor.ID,
		Model:     monitor.PrimaryModel,
		Status:    model.ChannelMonitorStatusDegraded,
		LatencyMs: 7200,
		CheckedAt: 200,
	}).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/channel-monitor", ListChannelMonitors)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/channel-monitor?p=1&page_size=10", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "ciphertext-recognizable-marker")
	assert.NotContains(t, recorder.Body.String(), "api_key_encrypted")

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				ID                   int      `json:"id"`
				CredentialConfigured bool     `json:"credential_configured"`
				CredentialMasked     string   `json:"credential_masked"`
				ExtraModels          []string `json:"extra_models"`
				LatestStatus         string   `json:"latest_status"`
				LatestLatencyMs      *int64   `json:"latest_latency_ms"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, monitor.ID, response.Data.Items[0].ID)
	assert.True(t, response.Data.Items[0].CredentialConfigured)
	assert.Equal(t, "****last", response.Data.Items[0].CredentialMasked)
	assert.Equal(t, []string{"gpt-test-mini"}, response.Data.Items[0].ExtraModels)
	assert.Equal(t, model.ChannelMonitorStatusDegraded, response.Data.Items[0].LatestStatus)
	require.NotNil(t, response.Data.Items[0].LatestLatencyMs)
	assert.Equal(t, int64(7200), *response.Data.Items[0].LatestLatencyMs)
}

func TestCreateChannelMonitorStoresCreatorAndReturnsMaskedCredential(t *testing.T) {
	setupChannelMonitorControllerTestDB(t)
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "channel-monitor-controller-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	body, err := common.Marshal(map[string]any{
		"internal_name": "internal monitor",
		"public_name":   "Public Monitor",
		"provider":      model.ChannelMonitorProviderOpenAI,
		"api_mode":      model.ChannelMonitorAPIModeResponses,
		"endpoint":      "https://example.com/",
		"api_key":       "monitor-create-secret",
		"primary_model": "gpt-test",
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("id", 42)
		c.Next()
	})
	engine.POST("/channel-monitor", CreateChannelMonitor)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/channel-monitor", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "monitor-create-secret")
	assert.NotContains(t, recorder.Body.String(), "api_key_encrypted")

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID                   int    `json:"id"`
			Endpoint             string `json:"endpoint"`
			CredentialConfigured bool   `json:"credential_configured"`
			CredentialMasked     string `json:"credential_masked"`
			CreatedBy            int    `json:"created_by"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, "https://example.com", response.Data.Endpoint)
	assert.True(t, response.Data.CredentialConfigured)
	assert.Equal(t, "****cret", response.Data.CredentialMasked)
	assert.Equal(t, 42, response.Data.CreatedBy)

	stored, err := model.GetChannelMonitor(response.Data.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, 42, stored.CreatedBy)
	assert.NotEmpty(t, stored.APIKeyEncrypted)
	assert.NotContains(t, stored.APIKeyEncrypted, "monitor-create-secret")
}

func TestRunChannelMonitorNowReturnsConflictForActiveLease(t *testing.T) {
	setupChannelMonitorControllerTestDB(t)
	monitor := &model.ChannelMonitor{
		InternalName:        "internal monitor",
		PublicName:          "Public Monitor",
		Provider:            model.ChannelMonitorProviderOpenAI,
		APIMode:             model.ChannelMonitorAPIModeChatCompletions,
		Endpoint:            "https://example.com",
		APIKeyEncrypted:     "ciphertext",
		PrimaryModel:        "gpt-test",
		ExtraModels:         "[]",
		IntervalSeconds:     60,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
		Enabled:             true,
		NextCheckAt:         100,
		LeaseOwner:          "active-owner",
		LeaseUntil:          time.Now().Unix() + 600,
	}
	require.NoError(t, model.CreateChannelMonitor(monitor))

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/channel-monitor/:id/run", RunChannelMonitorNow)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/channel-monitor/"+strconv.Itoa(monitor.ID)+"/run", nil))

	assert.Equal(t, http.StatusConflict, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.NotEmpty(t, response.Message)
}

func TestChannelMonitorHistorySupportsBoundedFilters(t *testing.T) {
	setupChannelMonitorControllerTestDB(t)
	monitor := channelMonitorControllerFixture()
	require.NoError(t, model.CreateChannelMonitor(monitor))
	require.NoError(t, model.DB.Create(&[]model.ChannelMonitorCheck{
		{MonitorID: monitor.ID, Model: "model-a", Status: model.ChannelMonitorStatusFailed, ErrorCategory: model.ChannelMonitorErrorAuth, HTTPStatus: http.StatusUnauthorized, ErrorMessage: "auth failed", CheckedAt: 300},
		{MonitorID: monitor.ID, Model: "model-a", Status: model.ChannelMonitorStatusOperational, LatencyMs: 100, CheckedAt: 200},
		{MonitorID: monitor.ID, Model: "model-b", Status: model.ChannelMonitorStatusFailed, ErrorCategory: model.ChannelMonitorErrorAuth, HTTPStatus: http.StatusForbidden, ErrorMessage: "other model", CheckedAt: 250},
	}).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/channel-monitor/:id/history", GetChannelMonitorHistory)
	recorder := httptest.NewRecorder()
	requestURL := "/channel-monitor/" + strconv.Itoa(monitor.ID) + "/history?p=1&page_size=1&model=model-a&status=failed&error_category=auth&start_time=250&end_time=350"
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestURL, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Page     int                         `json:"page"`
			PageSize int                         `json:"page_size"`
			Total    int                         `json:"total"`
			Items    []model.ChannelMonitorCheck `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Page)
	assert.Equal(t, 1, response.Data.PageSize)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "model-a", response.Data.Items[0].Model)
	assert.Equal(t, http.StatusUnauthorized, response.Data.Items[0].HTTPStatus)
	assert.Equal(t, "auth failed", response.Data.Items[0].ErrorMessage)
}

func TestChannelMonitorControllersExposeOnlyPublishedSafeStatusData(t *testing.T) {
	setupChannelMonitorControllerTestDB(t)
	monitor := channelMonitorControllerFixture()
	monitor.InternalName = "SENSITIVE_INTERNAL"
	monitor.Endpoint = "https://sensitive.example.com"
	monitor.APIKeyEncrypted = "SENSITIVE_CIPHERTEXT"
	monitor.PublicEnabled = true
	monitor.LastCheckedAt = 300
	require.NoError(t, model.CreateChannelMonitor(monitor))
	require.NoError(t, model.DB.Create(&model.ChannelMonitorCheck{
		MonitorID: monitor.ID, Model: monitor.PrimaryModel, Status: model.ChannelMonitorStatusFailed,
		ErrorCategory: model.ChannelMonitorErrorAuth, HTTPStatus: http.StatusUnauthorized,
		ErrorMessage: "SENSITIVE_ERROR", CheckedAt: 300,
	}).Error)
	service.InvalidateChannelMonitorPublicSummaryCache()
	t.Cleanup(service.InvalidateChannelMonitorPublicSummaryCache)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/channel-monitor/public-summary", GetChannelMonitorPublicSummary)
	engine.GET("/channel-monitor/status", GetChannelMonitorStatus)
	engine.GET("/channel-monitor/status/:id", GetChannelMonitorStatusDetail)

	for _, path := range []string{
		"/channel-monitor/public-summary",
		"/channel-monitor/status?days=1",
		"/channel-monitor/status/" + strconv.Itoa(monitor.ID),
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		payload := recorder.Body.String()
		assert.NotContains(t, payload, "SENSITIVE_INTERNAL")
		assert.NotContains(t, payload, "sensitive.example.com")
		assert.NotContains(t, payload, "SENSITIVE_CIPHERTEXT")
		assert.NotContains(t, payload, "SENSITIVE_ERROR")
		assert.NotContains(t, payload, "http_status")
		assert.NotContains(t, payload, "error_message")
	}
}

func TestUpdateChannelMonitorInvalidatesPublicSummaryCache(t *testing.T) {
	setupChannelMonitorControllerTestDB(t)
	monitor := channelMonitorControllerFixture()
	monitor.PublicEnabled = true
	require.NoError(t, model.CreateChannelMonitor(monitor))
	service.InvalidateChannelMonitorPublicSummaryCache()
	t.Cleanup(service.InvalidateChannelMonitorPublicSummaryCache)
	before, err := service.GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	require.Len(t, before.Monitors, 1)

	body, err := common.Marshal(map[string]any{
		"internal_name":         monitor.InternalName,
		"public_name":           monitor.PublicName,
		"provider":              monitor.Provider,
		"api_mode":              monitor.APIMode,
		"endpoint":              monitor.Endpoint,
		"primary_model":         monitor.PrimaryModel,
		"extra_models":          []string{},
		"interval_seconds":      monitor.IntervalSeconds,
		"degraded_threshold_ms": monitor.DegradedThresholdMs,
		"timeout_seconds":       monitor.TimeoutSeconds,
		"enabled":               true,
		"public_enabled":        false,
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.PUT("/channel-monitor/:id", UpdateChannelMonitor)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/channel-monitor/"+strconv.Itoa(monitor.ID), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	after, err := service.GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	assert.Empty(t, after.Monitors)
}

func setupChannelMonitorControllerTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	service.InvalidateChannelMonitorPublicSummaryCache()
	require.NoError(t, db.AutoMigrate(
		&model.ChannelMonitor{},
		&model.ChannelMonitorCheck{},
		&model.ChannelMonitorHourlyRollup{},
		&model.ChannelMonitorDailyRollup{},
	))
	t.Cleanup(func() {
		service.InvalidateChannelMonitorPublicSummaryCache()
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
}

func channelMonitorControllerFixture() *model.ChannelMonitor {
	return &model.ChannelMonitor{
		InternalName:        "internal monitor",
		PublicName:          "Public Monitor",
		Provider:            model.ChannelMonitorProviderOpenAI,
		APIMode:             model.ChannelMonitorAPIModeChatCompletions,
		Endpoint:            "https://example.com",
		APIKeyEncrypted:     "ciphertext",
		APIKeySuffix:        "text",
		PrimaryModel:        "model-a",
		ExtraModels:         "[]",
		IntervalSeconds:     60,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
		Enabled:             true,
		NextCheckAt:         100,
	}
}
