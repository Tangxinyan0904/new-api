package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

func registerChannelMonitorRoutes(apiRouter *gin.RouterGroup) {
	channelMonitorRoute := apiRouter.Group("/channel-monitor")
	channelMonitorRoute.Use(middleware.AdminAuth())
	for _, route := range channelMonitorPermissionRoutes {
		channelMonitorRoute.Handle(
			route.method,
			route.path,
			middleware.RequirePermission(route.permission),
			route.handler,
		)
	}
}

var channelMonitorPermissionRoutes = []permissionRoute{
	{method: http.MethodGet, path: "/", permission: authz.ChannelRead, handler: controller.ListChannelMonitors},
	{method: http.MethodPost, path: "/", permission: authz.ChannelSensitiveWrite, handler: controller.CreateChannelMonitor},
	{method: http.MethodGet, path: "/:id", permission: authz.ChannelRead, handler: controller.GetChannelMonitor},
	{method: http.MethodPut, path: "/:id", permission: authz.ChannelSensitiveWrite, handler: controller.UpdateChannelMonitor},
	{method: http.MethodDelete, path: "/:id", permission: authz.ChannelSensitiveWrite, handler: controller.DeleteChannelMonitor},
}
