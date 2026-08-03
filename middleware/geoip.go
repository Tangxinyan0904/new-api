package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type GeoIPDecisionResolver func(c *gin.Context) service.GeoIPDecision

func GeoIPAccess() gin.HandlerFunc {
	return GeoIPAccessWithResolver(func(c *gin.Context) service.GeoIPDecision {
		return service.ResolveGeoIPDecision(c.ClientIP())
	})
}

func GeoIPAccessWithResolver(resolve GeoIPDecisionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isGeoIPStatusPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		decision := resolve(c)
		if !decision.Enabled || !decision.Blocked {
			c.Next()
			return
		}

		isAPI := isGeoIPAPIPath(c.Request.URL.Path)
		if (isAPI && decision.RejectsAPI()) || (!isAPI && decision.RejectsWeb()) {
			abortGeoIPRequest(c, decision, isAPI)
			return
		}
		c.Next()
	}
}

func isGeoIPStatusPath(requestPath string) bool {
	return requestPath == "/api/geoip/status"
}

func isGeoIPAPIPath(requestPath string) bool {
	if hasGeoIPPathPrefix(requestPath, "/api") {
		return true
	}
	for _, prefix := range []string{"/v1", "/v1beta", "/mj", "/suno", "/kling", "/jimeng"} {
		if hasGeoIPPathPrefix(requestPath, prefix) {
			return true
		}
	}
	return false
}

func hasGeoIPPathPrefix(requestPath string, prefix string) bool {
	return requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/")
}

func abortGeoIPRequest(c *gin.Context, decision service.GeoIPDecision, isAPI bool) {
	message := strings.TrimSpace(decision.Message)
	if message == "" {
		message = "request blocked by GeoIP access restriction"
	}
	if isAPI {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": message,
			"error": gin.H{
				"message": message,
				"type":    "geoip_access_restricted",
			},
		})
		return
	}
	c.String(http.StatusForbidden, message)
	c.Abort()
}
