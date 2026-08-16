package router

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelMonitorRoutesUseChannelPermissions(t *testing.T) {
	assertChannelMonitorRoutePermission(t, http.MethodGet, "/", authz.ChannelRead, controller.ListChannelMonitors)
	assertChannelMonitorRoutePermission(t, http.MethodPost, "/", authz.ChannelSensitiveWrite, controller.CreateChannelMonitor)
	assertChannelMonitorRoutePermission(t, http.MethodGet, "/:id", authz.ChannelRead, controller.GetChannelMonitor)
	assertChannelMonitorRoutePermission(t, http.MethodPut, "/:id", authz.ChannelSensitiveWrite, controller.UpdateChannelMonitor)
	assertChannelMonitorRoutePermission(t, http.MethodDelete, "/:id", authz.ChannelSensitiveWrite, controller.DeleteChannelMonitor)
	assertChannelMonitorRoutePermission(t, http.MethodPost, "/:id/run", authz.ChannelOperate, controller.RunChannelMonitorNow)
	assertChannelMonitorRoutePermission(t, http.MethodGet, "/:id/history", authz.ChannelRead, controller.GetChannelMonitorHistory)
}

func TestChannelMonitorRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api")

	require.NotPanics(t, func() {
		registerChannelMonitorRoutes(api)
	})
}

func TestChannelMonitorPublicSummaryIsAnonymousAndStatusRequiresUser(t *testing.T) {
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
	service.InvalidateChannelMonitorPublicSummaryCache()
	t.Cleanup(func() {
		service.InvalidateChannelMonitorPublicSummaryCache()
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api")
	registerChannelMonitorRoutes(api)

	publicRecorder := httptest.NewRecorder()
	engine.ServeHTTP(publicRecorder, httptest.NewRequest(http.MethodGet, "/api/channel-monitor/public-summary", nil))
	assert.Equal(t, http.StatusOK, publicRecorder.Code)

	statusRecorder := httptest.NewRecorder()
	engine.ServeHTTP(statusRecorder, httptest.NewRequest(http.MethodGet, "/api/channel-monitor/status?days=1", nil))
	assert.Equal(t, http.StatusUnauthorized, statusRecorder.Code)
}

func assertChannelMonitorRoutePermission(t *testing.T, method string, path string, permission authz.Permission, handler any) {
	t.Helper()
	for _, route := range channelMonitorPermissionRoutes {
		if route.method == method && route.path == path {
			assert.Equal(t, permission, route.permission)
			assert.Equal(t, reflect.ValueOf(handler).Pointer(), reflect.ValueOf(route.handler).Pointer())
			return
		}
	}
	t.Fatalf("channel monitor route %s %s not found", method, path)
}
