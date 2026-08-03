package controller

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type registrationIPControllerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func TestRegistrationIPPasswordCreatesFourthThenRejectsLaterRegistration(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)
	const clientIP = "203.0.113.40"

	for index := 1; index <= model.RegistrationIPAccountLimit+1; index++ {
		response := performPasswordRegistration(t, index, clientIP)
		if index <= model.RegistrationIPAccountLimit {
			assert.True(t, response.Success, response.Message)
			continue
		}
		assert.False(t, response.Success)
		assert.Contains(t, strings.ToLower(response.Message), "created")
		assert.Contains(t, strings.ToLower(response.Message), "disabled")
	}

	var users []model.User
	require.NoError(t, db.Order("id ASC").Find(&users).Error)
	require.Len(t, users, model.RegistrationIPAccountLimit+1)
	for _, user := range users {
		assert.Equal(t, common.UserStatusDisabled, user.Status)
	}
	var tokenCount int64
	require.NoError(t, db.Model(&model.Token{}).Count(&tokenCount).Error)
	assert.EqualValues(t, model.RegistrationIPAccountLimit+1, tokenCount)

	blockedResponse := performPasswordRegistration(t, 5, clientIP)
	assert.False(t, blockedResponse.Success)
	assert.Contains(t, strings.ToLower(blockedResponse.Message), "administrator")
	assert.Contains(t, strings.ToLower(blockedResponse.Message), "unblock")
	var userCount int64
	require.NoError(t, db.Model(&model.User{}).Count(&userCount).Error)
	assert.EqualValues(t, model.RegistrationIPAccountLimit+1, userCount)
	require.NoError(t, db.Model(&model.Token{}).Count(&tokenCount).Error)
	assert.EqualValues(t, model.RegistrationIPAccountLimit+1, tokenCount)
}

func TestRegistrationIPOAuthBuiltInAndCustomProvidersUseSharedRegistration(t *testing.T) {
	t.Run("built-in provider", func(t *testing.T) {
		db := setupRegistrationIPControllerTest(t)
		provider := &registrationIPTestOAuthProvider{}
		for index := 1; index <= model.RegistrationIPAccountLimit+1; index++ {
			user, err := performOAuthUserCreation(
				t,
				provider,
				&oauth.OAuthUser{
					ProviderUserID: fmt.Sprintf("built-in-%d", index),
					Username:       fmt.Sprintf("oauth-user-%d", index),
					DisplayName:    fmt.Sprintf("OAuth User %d", index),
				},
				"203.0.113.41",
			)
			if index <= model.RegistrationIPAccountLimit {
				require.NoError(t, err)
				require.NotNil(t, user)
				continue
			}
			require.ErrorIs(t, err, model.ErrRegistrationIPLimitExceeded)
		}

		var users []model.User
		require.NoError(t, db.Order("id ASC").Find(&users).Error)
		require.Len(t, users, model.RegistrationIPAccountLimit+1)
		for index, user := range users {
			assert.Equal(t, common.UserStatusDisabled, user.Status)
			assert.Equal(t, fmt.Sprintf("built-in-%d", index+1), user.GitHubId)
		}
	})

	t.Run("custom provider", func(t *testing.T) {
		db := setupRegistrationIPControllerTest(t)
		config := model.CustomOAuthProvider{
			Id:      71,
			Name:    "Test Custom OAuth",
			Slug:    "test-custom",
			Enabled: true,
		}
		require.NoError(t, db.Create(&config).Error)
		provider := oauth.NewGenericOAuthProvider(&config)
		for index := 1; index <= model.RegistrationIPAccountLimit+1; index++ {
			_, err := performOAuthUserCreation(
				t,
				provider,
				&oauth.OAuthUser{
					ProviderUserID: fmt.Sprintf("custom-%d", index),
					Username:       fmt.Sprintf("custom-user-%d", index),
				},
				"203.0.113.42",
			)
			if index <= model.RegistrationIPAccountLimit {
				require.NoError(t, err)
				continue
			}
			require.ErrorIs(t, err, model.ErrRegistrationIPLimitExceeded)
		}

		var bindingCount int64
		require.NoError(t, db.Model(&model.UserOAuthBinding{}).Count(&bindingCount).Error)
		assert.EqualValues(t, model.RegistrationIPAccountLimit+1, bindingCount)
		var enabledCount int64
		require.NoError(t, db.Model(&model.User{}).Where("status = ?", common.UserStatusEnabled).Count(&enabledCount).Error)
		assert.Zero(t, enabledCount)
	})
}

func TestRegistrationIPWeChatUsesSharedRegistration(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		payload, err := common.Marshal(wechatLoginResponse{
			Success: true,
			Data:    "wechat-" + request.URL.Query().Get("code"),
		})
		require.NoError(t, err)
		_, err = writer.Write(payload)
		require.NoError(t, err)
	}))
	defer server.Close()
	common.WeChatServerAddress = server.URL

	for index := 1; index <= model.RegistrationIPAccountLimit+1; index++ {
		recorder := performControllerRequest(
			t,
			http.MethodGet,
			"/api/oauth/wechat?code="+strconv.Itoa(index),
			"203.0.113.43:1234",
			WeChatAuth,
		)
		var response registrationIPControllerResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		if index <= model.RegistrationIPAccountLimit {
			assert.True(t, response.Success, response.Message)
		} else {
			assert.False(t, response.Success)
			assert.Contains(t, strings.ToLower(response.Message), "disabled")
		}
	}

	var enabledCount int64
	require.NoError(t, db.Model(&model.User{}).Where("status = ?", common.UserStatusEnabled).Count(&enabledCount).Error)
	assert.Zero(t, enabledCount)
	var accountCount int64
	require.NoError(t, db.Model(&model.RegistrationIPAccount{}).Count(&accountCount).Error)
	assert.EqualValues(t, model.RegistrationIPAccountLimit+1, accountCount)
}

func TestRegistrationIPTelegramCreatesUsersAndAppliesSharedLimit(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)
	common.TelegramBotToken = "registration-ip-telegram-token"

	for index := 1; index <= model.RegistrationIPAccountLimit+1; index++ {
		query := signedTelegramLoginQuery(common.TelegramBotToken, index)
		recorder := performControllerRequest(
			t,
			http.MethodGet,
			"/api/oauth/telegram/login?"+query.Encode(),
			"203.0.113.44:1234",
			TelegramLogin,
		)
		var response registrationIPControllerResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		if index <= model.RegistrationIPAccountLimit {
			assert.True(t, response.Success, response.Message)
		} else {
			assert.False(t, response.Success)
			assert.Contains(t, strings.ToLower(response.Message), "disabled")
		}
	}

	var users []model.User
	require.NoError(t, db.Order("id ASC").Find(&users).Error)
	require.Len(t, users, model.RegistrationIPAccountLimit+1)
	for index, user := range users {
		assert.Equal(t, common.UserStatusDisabled, user.Status)
		assert.Equal(t, strconv.Itoa(index+1), user.TelegramId)
	}
}

func TestRegistrationIPAdminCreationIsExcludedAndManualStatusClearsRestoreEligibility(t *testing.T) {
	db := setupRegistrationIPControllerTest(t)
	adminBody, err := common.Marshal(model.User{
		Username:    "admin-created",
		Password:    "password123",
		DisplayName: "Admin Created",
		Role:        common.RoleCommonUser,
	})
	require.NoError(t, err)
	adminRecorder := httptest.NewRecorder()
	adminContext, _ := gin.CreateTestContext(adminRecorder)
	adminContext.Request = httptest.NewRequest(http.MethodPost, "/api/user", bytes.NewReader(adminBody))
	adminContext.Set("id", 1)
	adminContext.Set("username", "root-admin")
	adminContext.Set("role", common.RoleAdminUser)
	CreateUser(adminContext)
	var adminResponse registrationIPControllerResponse
	require.NoError(t, common.Unmarshal(adminRecorder.Body.Bytes(), &adminResponse))
	assert.True(t, adminResponse.Success, adminResponse.Message)
	var associationCount int64
	require.NoError(t, db.Model(&model.RegistrationIPAccount{}).Count(&associationCount).Error)
	assert.Zero(t, associationCount)

	userIDs := createControllerBlockedRegistrationIPFixture(t, "203.0.113.45")
	for _, action := range []string{"disable", "enable"} {
		body, marshalErr := common.Marshal(ManageRequest{Id: userIDs[0], Action: action})
		require.NoError(t, marshalErr)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", bytes.NewReader(body))
		ctx.Set("id", 1)
		ctx.Set("username", "root-admin")
		ctx.Set("role", common.RoleRootUser)
		ManageUser(ctx)
		var response registrationIPControllerResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.True(t, response.Success, response.Message)

		var account model.RegistrationIPAccount
		require.NoError(t, db.Where("user_id = ?", userIDs[0]).First(&account).Error)
		assert.False(t, account.RestoreEligible)
		assert.Zero(t, account.AutoDisabledAt)
	}
}

func setupRegistrationIPControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, db.AutoMigrate(
		&model.Token{},
		&model.UserSession{},
		&model.AuthFlow{},
		&model.ExternalIdentityClaim{},
		&model.Log{},
		&model.RegistrationIPState{},
		&model.RegistrationIPAccount{},
		&model.CustomOAuthProvider{},
		&model.UserOAuthBinding{},
		&model.CasbinRule{},
		&model.AuthzRole{},
	))

	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	previousWeChatAuthEnabled := common.WeChatAuthEnabled
	previousWeChatServerAddress := common.WeChatServerAddress
	previousWeChatServerToken := common.WeChatServerToken
	previousTelegramOAuthEnabled := common.TelegramOAuthEnabled
	previousTelegramBotToken := common.TelegramBotToken
	previousGenerateDefaultToken := constant.GenerateDefaultToken
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.WeChatAuthEnabled = true
	common.WeChatServerToken = "test-wechat-token"
	common.TelegramOAuthEnabled = true
	constant.GenerateDefaultToken = true
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
		common.WeChatAuthEnabled = previousWeChatAuthEnabled
		common.WeChatServerAddress = previousWeChatServerAddress
		common.WeChatServerToken = previousWeChatServerToken
		common.TelegramOAuthEnabled = previousTelegramOAuthEnabled
		common.TelegramBotToken = previousTelegramBotToken
		constant.GenerateDefaultToken = previousGenerateDefaultToken
	})
	return db
}

func performPasswordRegistration(t *testing.T, index int, clientIP string) registrationIPControllerResponse {
	t.Helper()
	body, err := common.Marshal(model.User{
		Username: fmt.Sprintf("password-user-%d", index),
		Password: "password123",
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	ctx.Request.RemoteAddr = clientIP + ":1234"
	ctx.Request.Header.Set("Accept-Language", "en")
	Register(ctx)
	var response registrationIPControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func performOAuthUserCreation(
	t *testing.T,
	provider oauth.Provider,
	oauthUser *oauth.OAuthUser,
	clientIP string,
) (*model.User, error) {
	t.Helper()
	var user *model.User
	var creationErr error
	performControllerRequest(
		t,
		http.MethodGet,
		"/api/oauth/test",
		clientIP+":1234",
		func(c *gin.Context) {
			user, creationErr = findOrCreateOAuthUser(c, provider, oauthUser, "")
		},
	)
	return user, creationErr
}

func performControllerRequest(
	t *testing.T,
	method string,
	target string,
	remoteAddr string,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Handle(method, "/api/oauth/test", handler)
	router.Handle(method, "/api/oauth/wechat", handler)
	router.Handle(method, "/api/oauth/telegram/login", handler)
	request := httptest.NewRequest(method, target, nil)
	request.RemoteAddr = remoteAddr
	request.Header.Set("Accept-Language", "en")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func signedTelegramLoginQuery(token string, id int) url.Values {
	values := url.Values{
		"id":         {strconv.Itoa(id)},
		"username":   {fmt.Sprintf("telegram_user_%d", id)},
		"first_name": {"Telegram"},
		"last_name":  {strconv.Itoa(id)},
		"auth_date":  {strconv.FormatInt(time.Now().Unix(), 10)},
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	secretHash := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secretHash[:])
	_, _ = mac.Write([]byte(strings.Join(parts, "\n")))
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return values
}

func createControllerBlockedRegistrationIPFixture(t *testing.T, registrationIP string) []int {
	t.Helper()
	userIDs := make([]int, 0, model.RegistrationIPAccountLimit+1)
	for index := 1; index <= model.RegistrationIPAccountLimit+1; index++ {
		user := &model.User{
			Username:    fmt.Sprintf("managed-ip-user-%d", index),
			DisplayName: fmt.Sprintf("Managed IP User %d", index),
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
		}
		_, err := model.RegisterSelfServiceUser(user, 0, registrationIP, nil)
		require.NoError(t, err)
		userIDs = append(userIDs, user.Id)
	}
	return userIDs
}

type registrationIPTestOAuthProvider struct{}

func (provider *registrationIPTestOAuthProvider) GetName() string { return "Test OAuth" }
func (provider *registrationIPTestOAuthProvider) IsEnabled() bool { return true }
func (provider *registrationIPTestOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{}, nil
}
func (provider *registrationIPTestOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return nil, errors.New("not used")
}
func (provider *registrationIPTestOAuthProvider) IsUserIDTaken(string) bool { return false }
func (provider *registrationIPTestOAuthProvider) FillUserByProviderID(*model.User, string) error {
	return nil
}
func (provider *registrationIPTestOAuthProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}
func (provider *registrationIPTestOAuthProvider) GetProviderPrefix() string { return "oauth_" }
