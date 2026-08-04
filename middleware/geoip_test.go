package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/geoip_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeoIPMiddlewareRejectsBlockedAPIMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GeoIPAccessWithResolver(func(*gin.Context) service.GeoIPDecision {
		return service.GeoIPDecision{
			Enabled:       true,
			Mode:          geoip_setting.ModeHomepageBlockAPIReject,
			Blocked:       true,
			Message:       "blocked",
			DatabaseReady: true,
		}
	}))
	router.GET("/api/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "blocked")
}

func TestGeoIPMiddlewareExemptsStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GeoIPAccessWithResolver(func(*gin.Context) service.GeoIPDecision {
		return service.GeoIPDecision{
			Enabled:       true,
			Mode:          geoip_setting.ModeFullReject,
			Blocked:       true,
			Message:       "blocked",
			DatabaseReady: true,
		}
	}))
	router.GET("/api/geoip/status", func(c *gin.Context) { c.Status(http.StatusOK) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/geoip/status", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGeoIPMiddlewareAllowsWebPathInAPIRejectMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GeoIPAccessWithResolver(func(*gin.Context) service.GeoIPDecision {
		return service.GeoIPDecision{
			Enabled:       true,
			Mode:          geoip_setting.ModeHomepageBlockAPIReject,
			Blocked:       true,
			Message:       "blocked",
			DatabaseReady: true,
		}
	}))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGeoIPMiddlewareClassifiesRelayRoutePatternsAsAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GeoIPAccessWithResolver(func(*gin.Context) service.GeoIPDecision {
		return service.GeoIPDecision{
			Enabled:       true,
			Mode:          geoip_setting.ModeHomepageBlockAPIReject,
			Blocked:       true,
			Message:       "blocked",
			DatabaseReady: true,
		}
	}))
	router.POST("/pg/chat/completions", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.POST("/:mode/mj/submit/:action", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/docs/:slug", func(c *gin.Context) { c.Status(http.StatusOK) })

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "playground relay", method: http.MethodPost, path: "/pg/chat/completions", wantStatus: http.StatusForbidden},
		{name: "mode midjourney relay", method: http.MethodPost, path: "/fast/mj/submit/imagine", wantStatus: http.StatusForbidden},
		{name: "ordinary dynamic web route", method: http.MethodGet, path: "/docs/getting-started", wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			require.Equal(t, test.wantStatus, recorder.Code)
		})
	}
}
