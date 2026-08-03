package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatusIncludesAPIKeyNotice(t *testing.T) {
	original := console_setting.GetConsoleSetting().ApiKeyNotice
	console_setting.GetConsoleSetting().ApiKeyNotice = "Keep this key private.\nRotate it regularly."
	t.Cleanup(func() {
		console_setting.GetConsoleSetting().ApiKeyNotice = original
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/status", GetStatus)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/status", nil)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(
		t,
		"Keep this key private.\nRotate it regularly.",
		response.Data["api_key_notice"],
	)
}
