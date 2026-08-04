package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/geoip_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNormalizeGeoIPOptionValueNormalizesBlockedCountries(t *testing.T) {
	got, err := normalizeGeoIPOptionValue("geoip.blocked_countries", `["cn"," US ","cn"]`)

	require.NoError(t, err)
	require.JSONEq(t, `["CN","US"]`, got)
}

func TestNormalizeGeoIPOptionValueRejectsInvalidMode(t *testing.T) {
	_, err := normalizeGeoIPOptionValue("geoip.mode", "unknown")

	require.Error(t, err)
}

func TestNormalizeGeoIPOptionValueAcceptsValidMode(t *testing.T) {
	got, err := normalizeGeoIPOptionValue("geoip.mode", geoip_setting.ModeFullReject)

	require.NoError(t, err)
	require.Equal(t, geoip_setting.ModeFullReject, got)
}

func TestIsSensitiveOptionKeyHidesLowercaseGeoIPKey(t *testing.T) {
	require.True(t, isSensitiveOptionKey("geoip.maxmind_license_key"))
	require.True(t, isSensitiveOptionKey("StripeApiSecret"))
	require.False(t, isSensitiveOptionKey("geoip.download_url"))
}

func TestUpdateGeoIPOptionsPersistsNormalizedSettingsTogether(t *testing.T) {
	db := setupGeoIPOptionControllerTest(t)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/geoip", strings.NewReader(`{
		"geoip.mode":"full_reject",
		"geoip.database_path":"GeoLite2-Country.mmdb",
		"geoip.download_url":"https://example.com/geoip.zip",
		"geoip.maxmind_license_key":"license-key",
		"geoip.popup_message":"Unavailable in this region",
		"geoip.allow_private_loopback":false,
		"geoip.blocked_countries":["us"," CN ","us"]
	}`))

	UpdateGeoIPOptions(context)

	assert.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)

	want := map[string]string{
		"geoip.mode":                   geoip_setting.ModeFullReject,
		"geoip.database_path":          "GeoLite2-Country.mmdb",
		"geoip.download_url":           "https://example.com/geoip.zip",
		"geoip.maxmind_license_key":    "license-key",
		"geoip.popup_message":          "Unavailable in this region",
		"geoip.allow_private_loopback": "false",
		"geoip.blocked_countries":      `["CN","US"]`,
	}
	var options []model.Option
	require.NoError(t, db.Order("key").Find(&options).Error)
	require.Len(t, options, len(want))
	for _, option := range options {
		assert.Equal(t, want[option.Key], option.Value, option.Key)
	}
	assert.Equal(t, geoip_setting.ModeFullReject, geoip_setting.Mode)
	assert.Equal(t, []string{"CN", "US"}, geoip_setting.BlockedCountries)
	assert.False(t, geoip_setting.AllowPrivateLoopback)
}

func TestUpdateGeoIPOptionsRejectsInvalidDependenciesBeforeEnablingMode(t *testing.T) {
	db := setupGeoIPOptionControllerTest(t)
	require.NoError(t, db.Create(&model.Option{Key: "geoip.mode", Value: geoip_setting.ModeOff}).Error)
	require.NoError(t, db.Create(&model.Option{Key: "geoip.blocked_countries", Value: `["CN"]`}).Error)

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/geoip", strings.NewReader(`{
		"geoip.mode":"full_reject",
		"geoip.database_path":"Country.mmdb",
		"geoip.download_url":"",
		"geoip.popup_message":"Blocked",
		"geoip.allow_private_loopback":true,
		"geoip.blocked_countries":["INVALID"]
	}`))

	UpdateGeoIPOptions(context)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Contains(t, payload.Message, "ISO 3166-1")
	assert.Equal(t, geoip_setting.ModeOff, geoip_setting.Mode)
	assert.Equal(t, []string{"CN"}, geoip_setting.BlockedCountries)

	var mode model.Option
	require.NoError(t, db.First(&mode, "key = ?", "geoip.mode").Error)
	assert.Equal(t, geoip_setting.ModeOff, mode.Value)
	var countries model.Option
	require.NoError(t, db.First(&countries, "key = ?", "geoip.blocked_countries").Error)
	assert.JSONEq(t, `["CN"]`, countries.Value)
}

func setupGeoIPOptionControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.User{}, &model.Log{}))

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMode := geoip_setting.Mode
	previousDatabasePath := geoip_setting.DatabasePath
	previousDownloadURL := geoip_setting.DownloadURL
	previousLicenseKey := geoip_setting.MaxMindLicenseKey
	previousPopupMessage := geoip_setting.PopupMessage
	previousAllowPrivateLoopback := geoip_setting.AllowPrivateLoopback
	previousBlockedCountries := append([]string(nil), geoip_setting.BlockedCountries...)
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	geoip_setting.Mode = geoip_setting.ModeOff
	geoip_setting.DatabasePath = "Country.mmdb"
	geoip_setting.DownloadURL = ""
	geoip_setting.MaxMindLicenseKey = ""
	geoip_setting.PopupMessage = geoip_setting.DefaultPopupMessage
	geoip_setting.AllowPrivateLoopback = true
	geoip_setting.BlockedCountries = []string{"CN"}

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		geoip_setting.Mode = previousMode
		geoip_setting.DatabasePath = previousDatabasePath
		geoip_setting.DownloadURL = previousDownloadURL
		geoip_setting.MaxMindLicenseKey = previousLicenseKey
		geoip_setting.PopupMessage = previousPopupMessage
		geoip_setting.AllowPrivateLoopback = previousAllowPrivateLoopback
		geoip_setting.BlockedCountries = previousBlockedCountries
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	return db
}
