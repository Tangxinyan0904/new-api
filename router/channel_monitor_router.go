package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

func registerChannelMonitorRoutes(apiRouter *gin.RouterGroup) {
	apiRouter.GET("/channel-monitor/public-summary", controller.GetChannelStatusPublicSummary)

	channelMonitorStatusRoute := apiRouter.Group("/channel-monitor/status")
	channelMonitorStatusRoute.Use(middleware.UserAuth())
	channelMonitorStatusRoute.GET("", controller.GetChannelStatus)
	channelMonitorStatusRoute.GET("/:id", controller.GetChannelStatusDetail)

	channelMonitorRoute := apiRouter.Group("/channel-monitor")
	channelMonitorRoute.Use(middleware.AdminAuth())
	handlePermissionRoute(channelMonitorRoute, http.MethodPost, "/", authz.ChannelWrite, controller.CreateChannelStatusMonitor)
	handlePermissionRoute(channelMonitorRoute, http.MethodPut, "/:id", authz.ChannelWrite, controller.UpdateChannelStatusMonitor)
	handlePermissionRoute(channelMonitorRoute, http.MethodDelete, "/:id", authz.ChannelWrite, controller.DeleteChannelStatusMonitor)
}
