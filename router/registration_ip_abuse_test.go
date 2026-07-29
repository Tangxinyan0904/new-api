package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistrationIPAbuseRoutesRequireRootAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("registration-ip-router-test"))))
	engine.GET("/test-login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "admin-user")
		session.Set("role", common.RoleAdminUser)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	SetApiRouter(engine)

	wantRoutes := map[string]bool{
		http.MethodGet + " /api/registration-ip-abuse/blocked":          false,
		http.MethodPost + " /api/registration-ip-abuse/:ip/unblock":     false,
		http.MethodGet + " /api/registration-ip-abuse/allowlist":        false,
		http.MethodPost + " /api/registration-ip-abuse/allowlist":       false,
		http.MethodDelete + " /api/registration-ip-abuse/allowlist/:ip": false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantRoutes[key]; ok {
			wantRoutes[key] = true
		}
	}
	for route, found := range wantRoutes {
		assert.True(t, found, route)
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodGet, "/test-login", nil)
	engine.ServeHTTP(loginRecorder, loginRequest)
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)
	require.NotEmpty(t, loginRecorder.Result().Cookies())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/registration-ip-abuse/blocked", nil)
	request.AddCookie(loginRecorder.Result().Cookies()[0])
	request.Header.Set("New-Api-User", strconv.Itoa(1))
	request.Header.Set("Accept-Language", "en")
	engine.ServeHTTP(recorder, request)

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "insufficient privileges")
}
