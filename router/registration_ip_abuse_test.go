package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRegistrationIPAbuseRoutesRequireRootAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousSessionSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.SessionSecret = "registration-ip-router-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		common.SessionSecret = previousSessionSecret
	})

	admin := &model.User{
		Username:    "registration-ip-admin",
		Password:    "password-placeholder",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(admin).Error)
	session, err := service.CreateLoginSession(admin.Id, "password", "127.0.0.1", "router-test")
	require.NoError(t, err)

	engine := gin.New()
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

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/registration-ip-abuse/blocked", nil)
	request.Header.Set("Authorization", "Bearer "+session.AccessToken)
	request.Header.Set("Accept-Language", "en")
	engine.ServeHTTP(recorder, request)

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "insufficient privileges")
}
