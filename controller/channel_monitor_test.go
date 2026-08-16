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

func setupChannelMonitorControllerTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(
		&model.ChannelMonitor{},
		&model.ChannelMonitorCheck{},
		&model.ChannelMonitorHourlyRollup{},
		&model.ChannelMonitorDailyRollup{},
	))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
}
