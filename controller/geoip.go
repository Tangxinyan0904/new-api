package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/geoip_setting"
	"github.com/gin-gonic/gin"
)

func GetGeoIPStatus(c *gin.Context) {
	common.ApiSuccess(c, service.ResolveGeoIPDecision(c.ClientIP()))
}

func DownloadGeoIPDatabase(c *gin.Context) {
	if err := service.DownloadGeoIPDatabase(c.Request.Context()); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"database_path": geoip_setting.DatabasePath,
	})
}
