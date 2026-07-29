package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type registrationIPAdminResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestRegistrationIPAbuseListsBlockedIPsByAssociatedUser(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)
	const registrationIP = "203.0.113.70"
	userIDs := createControllerBlockedRegistrationIPFixture(t, registrationIP)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", userIDs[0]).Updates(map[string]interface{}{
		"username":     "blocked-search-user",
		"display_name": "Blocked Search Display",
	}).Error)

	recorder := performRegistrationIPAdminRequest(
		t,
		http.MethodGet,
		"/api/registration-ip-abuse/blocked?p=1&page_size=1&keyword=blocked-search-user",
		nil,
		nil,
		ListBlockedRegistrationIPs,
	)

	var response registrationIPAdminResponse[struct {
		Page     int                                    `json:"page"`
		PageSize int                                    `json:"page_size"`
		Total    int                                    `json:"total"`
		Items    []*model.BlockedRegistrationIPListItem `json:"items"`
	}]
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success, response.Message)
	assert.Equal(t, 1, response.Data.Page)
	assert.Equal(t, 1, response.Data.PageSize)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, registrationIP, response.Data.Items[0].IP)
	assert.Equal(t, model.RegistrationIPAccountLimit+1, response.Data.Items[0].AssociatedAccountCount)
	require.Len(t, response.Data.Items[0].Accounts, model.RegistrationIPAccountLimit+1)
}

func TestRegistrationIPAbuseAllowlistListsCanonicalIPsWithPagination(t *testing.T) {
	setupRegistrationIPControllerTest(t)
	for _, registrationIP := range []string{"2001:db8::2", "203.0.113.72"} {
		_, err := model.AddRegistrationIPAllowlist(registrationIP)
		require.NoError(t, err)
	}

	recorder := performRegistrationIPAdminRequest(
		t,
		http.MethodGet,
		"/api/registration-ip-abuse/allowlist?p=1&page_size=1",
		nil,
		nil,
		ListRegistrationIPAllowlist,
	)

	var response registrationIPAdminResponse[struct {
		Page     int                                  `json:"page"`
		PageSize int                                  `json:"page_size"`
		Total    int                                  `json:"total"`
		Items    []*model.RegistrationIPAllowlistItem `json:"items"`
	}]
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success, response.Message)
	assert.Equal(t, 2, response.Data.Total)
	assert.Equal(t, 1, response.Data.PageSize)
	require.Len(t, response.Data.Items, 1)
	assert.NotEmpty(t, response.Data.Items[0].IP)
}

func TestRegistrationIPAbuseMutationsAreIdempotentAndRecordStructuredAudits(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)
	const registrationIP = "203.0.113.73"
	userIDs := createControllerBlockedRegistrationIPFixture(t, registrationIP)

	firstAdd := performRegistrationIPAdminRequest(
		t,
		http.MethodPost,
		"/api/registration-ip-abuse/allowlist",
		gin.H{"ip": "::ffff:" + registrationIP},
		nil,
		AddRegistrationIPAllowlist,
	)
	var firstAddResponse registrationIPAdminResponse[model.RegistrationIPMutationResult]
	require.NoError(t, common.Unmarshal(firstAdd.Body.Bytes(), &firstAddResponse))
	assert.True(t, firstAddResponse.Success, firstAddResponse.Message)
	assert.Equal(t, registrationIP, firstAddResponse.Data.CanonicalIP)
	assert.True(t, firstAddResponse.Data.Allowlisted)
	assert.ElementsMatch(t, userIDs, firstAddResponse.Data.AffectedUserIDs)

	secondAdd := performRegistrationIPAdminRequest(
		t,
		http.MethodPost,
		"/api/registration-ip-abuse/allowlist",
		gin.H{"ip": registrationIP},
		nil,
		AddRegistrationIPAllowlist,
	)
	var secondAddResponse registrationIPAdminResponse[model.RegistrationIPMutationResult]
	require.NoError(t, common.Unmarshal(secondAdd.Body.Bytes(), &secondAddResponse))
	assert.True(t, secondAddResponse.Success, secondAddResponse.Message)
	assert.Empty(t, secondAddResponse.Data.AffectedUserIDs)

	removed := performRegistrationIPAdminRequest(
		t,
		http.MethodDelete,
		"/api/registration-ip-abuse/allowlist/"+registrationIP,
		nil,
		gin.Params{{Key: "ip", Value: registrationIP}},
		RemoveRegistrationIPAllowlist,
	)
	var removedResponse registrationIPAdminResponse[model.RegistrationIPMutationResult]
	require.NoError(t, common.Unmarshal(removed.Body.Bytes(), &removedResponse))
	assert.True(t, removedResponse.Success, removedResponse.Message)
	assert.False(t, removedResponse.Data.Allowlisted)

	var auditLogs []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).Order("id ASC").Find(&auditLogs).Error)
	require.Len(t, auditLogs, 3)
	wantActions := []string{
		"registration_ip.allowlist_add",
		"registration_ip.allowlist_add",
		"registration_ip.allowlist_remove",
	}
	for index, auditLog := range auditLogs {
		other, err := common.StrToMap(auditLog.Other)
		require.NoError(t, err)
		op, ok := other["op"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, wantActions[index], op["action"])
		params, ok := op["params"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, registrationIP, params["ip"])
		assert.NotNil(t, params["affected_account_count"])
		assert.NotNil(t, params["affected_user_ids"])
		assert.NotNil(t, params["allowlisted"])
	}
}

func TestRegistrationIPAbuseUnblockRestoresEligibleAccountsAndRecordsAudit(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)
	const registrationIP = "203.0.113.74"
	userIDs := createControllerBlockedRegistrationIPFixture(t, registrationIP)

	recorder := performRegistrationIPAdminRequest(
		t,
		http.MethodPost,
		"/api/registration-ip-abuse/"+registrationIP+"/unblock",
		nil,
		gin.Params{{Key: "ip", Value: registrationIP}},
		UnblockRegistrationIP,
	)

	var response registrationIPAdminResponse[model.RegistrationIPMutationResult]
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success, response.Message)
	assert.Equal(t, registrationIP, response.Data.CanonicalIP)
	assert.ElementsMatch(t, userIDs, response.Data.AffectedUserIDs)
	assert.False(t, response.Data.Allowlisted)

	var auditLog model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).First(&auditLog).Error)
	assert.Contains(t, auditLog.Content, registrationIP)
	other, err := common.StrToMap(auditLog.Other)
	require.NoError(t, err)
	op, ok := other["op"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "registration_ip.unblock", op["action"])
}

func TestRegistrationIPAbuseRejectsInvalidAndCIDRInputsWithoutAudit(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)

	for _, registrationIP := range []string{"", "203.0.113.0/24", "example.com"} {
		recorder := performRegistrationIPAdminRequest(
			t,
			http.MethodPost,
			"/api/registration-ip-abuse/allowlist",
			gin.H{"ip": registrationIP},
			nil,
			AddRegistrationIPAllowlist,
		)
		var response registrationIPAdminResponse[model.RegistrationIPMutationResult]
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.False(t, response.Success)
	}

	invalidPath := performRegistrationIPAdminRequest(
		t,
		http.MethodPost,
		"/api/registration-ip-abuse/not-an-ip/unblock",
		nil,
		gin.Params{{Key: "ip", Value: "not-an-ip"}},
		UnblockRegistrationIP,
	)
	var invalidPathResponse registrationIPAdminResponse[model.RegistrationIPMutationResult]
	require.NoError(t, common.Unmarshal(invalidPath.Body.Bytes(), &invalidPathResponse))
	assert.False(t, invalidPathResponse.Success)

	var auditCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeManage).Count(&auditCount).Error)
	assert.Zero(t, auditCount)
}

func performRegistrationIPAdminRequest(
	t *testing.T,
	method string,
	target string,
	body any,
	params gin.Params,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		requestBody = bytes.NewReader(payload)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	ctx.Request.RemoteAddr = "198.51.100.10:1234"
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = params
	ctx.Set("id", 1)
	ctx.Set("username", "root-admin")
	ctx.Set("role", common.RoleRootUser)
	handler(ctx)
	return recorder
}
