package router

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorRoutesUseChannelPermissions(t *testing.T) {
	assertChannelMonitorRoutePermission(t, http.MethodGet, "/", authz.ChannelRead, controller.ListChannelMonitors)
	assertChannelMonitorRoutePermission(t, http.MethodPost, "/", authz.ChannelSensitiveWrite, controller.CreateChannelMonitor)
	assertChannelMonitorRoutePermission(t, http.MethodGet, "/:id", authz.ChannelRead, controller.GetChannelMonitor)
	assertChannelMonitorRoutePermission(t, http.MethodPut, "/:id", authz.ChannelSensitiveWrite, controller.UpdateChannelMonitor)
	assertChannelMonitorRoutePermission(t, http.MethodDelete, "/:id", authz.ChannelSensitiveWrite, controller.DeleteChannelMonitor)
}

func TestChannelMonitorRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api")

	require.NotPanics(t, func() {
		registerChannelMonitorRoutes(api)
	})
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
