package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupModelRequestRateLimitTest(t *testing.T, groups map[string][2]int, users map[int][2]int) {
	t.Helper()

	originalRedisEnabled := common.RedisEnabled
	originalEnabled := setting.ModelRequestRateLimitEnabled
	originalDuration := setting.ModelRequestRateLimitDurationMinutes
	originalCount := setting.ModelRequestRateLimitCount
	originalSuccessCount := setting.ModelRequestRateLimitSuccessCount
	originalGroups := setting.ModelRequestRateLimitGroup2JSONString()
	originalUsers := setting.ModelRequestRateLimitUser2JSONString()

	groupJSON, err := common.Marshal(groups)
	require.NoError(t, err)
	userJSON, err := common.Marshal(users)
	require.NoError(t, err)
	require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(string(groupJSON)))
	require.NoError(t, setting.UpdateModelRequestRateLimitUserByJSONString(string(userJSON)))

	common.RedisEnabled = false
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitDurationMinutes = 1
	inMemoryRateLimiter = common.InMemoryRateLimiter{}
	gin.SetMode(gin.TestMode)

	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		setting.ModelRequestRateLimitEnabled = originalEnabled
		setting.ModelRequestRateLimitDurationMinutes = originalDuration
		setting.ModelRequestRateLimitCount = originalCount
		setting.ModelRequestRateLimitSuccessCount = originalSuccessCount
		require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(originalGroups))
		require.NoError(t, setting.UpdateModelRequestRateLimitUserByJSONString(originalUsers))
		inMemoryRateLimiter = common.InMemoryRateLimiter{}
	})
}

func newModelRequestRateLimitRouter(userID int, group string) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", userID)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		c.Next()
	})
	router.Use(ModelRequestRateLimit())
	router.GET("/v1/models", func(c *gin.Context) {
		if c.GetHeader("X-Test-Failure") == "true" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	return router
}

func performModelRateLimitRequest(router *gin.Engine, failed bool) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	if failed {
		request.Header.Set("X-Test-Failure", "true")
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestModelRequestRateLimitUsesUserRuleBeforeGroupRule(t *testing.T) {
	setupModelRequestRateLimitTest(
		t,
		map[string][2]int{"vip": {3, 3}},
		map[int][2]int{42: {1, 3}},
	)
	setting.ModelRequestRateLimitCount = 5
	setting.ModelRequestRateLimitSuccessCount = 5
	router := newModelRequestRateLimitRouter(42, "vip")

	assert.Equal(t, http.StatusOK, performModelRateLimitRequest(router, false).Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, false).Code)
}

func TestModelRequestRateLimitRecordsOnlySuccessfulResponses(t *testing.T) {
	setupModelRequestRateLimitTest(t, map[string][2]int{}, map[int][2]int{})
	setting.ModelRequestRateLimitCount = 0
	setting.ModelRequestRateLimitSuccessCount = 1
	router := newModelRequestRateLimitRouter(7, "standard")

	assert.Equal(t, http.StatusInternalServerError, performModelRateLimitRequest(router, true).Code)
	assert.Equal(t, http.StatusOK, performModelRateLimitRequest(router, false).Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, false).Code)
}
