package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/geoip_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetGeoIPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/geoip/status", GetGeoIPStatus)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/geoip/status", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"enabled":false`)
}

func TestDownloadGeoIPDatabaseReturnsErrorWhenURLMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDownloadURL := geoip_setting.DownloadURL
	previousLicenseKey := geoip_setting.MaxMindLicenseKey
	geoip_setting.DownloadURL = ""
	geoip_setting.MaxMindLicenseKey = ""
	t.Cleanup(func() {
		geoip_setting.DownloadURL = previousDownloadURL
		geoip_setting.MaxMindLicenseKey = previousLicenseKey
	})

	router := gin.New()
	router.POST("/api/option/geoip/download", DownloadGeoIPDatabase)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/option/geoip/download", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
}
