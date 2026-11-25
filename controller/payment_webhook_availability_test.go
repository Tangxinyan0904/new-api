package controller

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func confirmPaymentComplianceForTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
}

func TestStripeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	t.Cleanup(func() {
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
	})

	setting.StripeWebhookSecret = ""
	setting.StripeApiSecret = "sk_test_123"
	setting.StripePriceId = "price_123"
	require.False(t, isStripeWebhookEnabled())

	setting.StripeWebhookSecret = "whsec_test"
	require.True(t, isStripeWebhookEnabled())

	setting.StripePriceId = ""
	require.False(t, isStripeWebhookEnabled())
}

func TestCreemWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPIKey := setting.CreemApiKey
	originalProducts := setting.CreemProducts
	originalWebhookSecret := setting.CreemWebhookSecret
	t.Cleanup(func() {
		setting.CreemApiKey = originalAPIKey
		setting.CreemProducts = originalProducts
		setting.CreemWebhookSecret = originalWebhookSecret
	})

	setting.CreemWebhookSecret = ""
	setting.CreemApiKey = "creem_api_key"
	setting.CreemProducts = `[{"productId":"prod_123"}]`
	require.False(t, isCreemWebhookEnabled())

	setting.CreemWebhookSecret = "creem_secret"
	require.True(t, isCreemWebhookEnabled())

	setting.CreemProducts = "[]"
	require.False(t, isCreemWebhookEnabled())
}

func TestWaffoWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.WaffoEnabled
	originalSandbox := setting.WaffoSandbox
	originalAPIKey := setting.WaffoApiKey
	originalPrivateKey := setting.WaffoPrivateKey
	originalPublicCert := setting.WaffoPublicCert
	originalSandboxAPIKey := setting.WaffoSandboxApiKey
	originalSandboxPrivateKey := setting.WaffoSandboxPrivateKey
	originalSandboxPublicCert := setting.WaffoSandboxPublicCert
	t.Cleanup(func() {
		setting.WaffoEnabled = originalEnabled
		setting.WaffoSandbox = originalSandbox
		setting.WaffoApiKey = originalAPIKey
		setting.WaffoPrivateKey = originalPrivateKey
		setting.WaffoPublicCert = originalPublicCert
		setting.WaffoSandboxApiKey = originalSandboxAPIKey
		setting.WaffoSandboxPrivateKey = originalSandboxPrivateKey
		setting.WaffoSandboxPublicCert = originalSandboxPublicCert
	})

	setting.WaffoEnabled = true
	setting.WaffoSandbox = false
	setting.WaffoApiKey = ""
	setting.WaffoPrivateKey = "private"
	setting.WaffoPublicCert = "public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoApiKey = "api"
	require.True(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = false
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = true
	setting.WaffoSandbox = true
	setting.WaffoSandboxApiKey = ""
	setting.WaffoSandboxPrivateKey = "sandbox_private"
	setting.WaffoSandboxPublicCert = "sandbox_public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoSandboxApiKey = "sandbox_api"
	require.True(t, isWaffoWebhookEnabled())
}

func TestWaffoPancakeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalMerchantID := setting.WaffoPancakeMerchantID
	originalPrivateKey := setting.WaffoPancakePrivateKey
	originalProductID := setting.WaffoPancakeProductID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID = originalMerchantID
		setting.WaffoPancakePrivateKey = originalPrivateKey
		setting.WaffoPancakeProductID = originalProductID
	})

	// Presence of all three credentials enables the gateway. Webhook public
	// keys are bundled in the SDK and there is no separate Enabled toggle —
	// clear any of the three fields to disable.
	setting.WaffoPancakeMerchantID = ""
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeProductID = "product"
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeMerchantID = "merchant"
	require.True(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = ""
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = "product"
	setting.WaffoPancakePrivateKey = ""
	require.False(t, isWaffoPancakeWebhookEnabled())
}

func TestEpayWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	paymentSetting := operation_setting.GetPaymentSetting()
	originalSignType := paymentSetting.EpaySignType
	originalMerchantPrivateKey := paymentSetting.EpayMerchantPrivateKey
	originalPlatformPublicKey := paymentSetting.EpayPlatformPublicKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		paymentSetting.EpaySignType = originalSignType
		paymentSetting.EpayMerchantPrivateKey = originalMerchantPrivateKey
		paymentSetting.EpayPlatformPublicKey = originalPlatformPublicKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = ""
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeMD5
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	require.False(t, isEpayWebhookEnabled())

	operation_setting.EpayKey = "epay_key"
	require.True(t, isEpayWebhookEnabled())

	operation_setting.PayMethods = nil
	require.False(t, isEpayWebhookEnabled())

	// RSA needs both keys, and the MD5 key no longer counts.
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeRSA
	paymentSetting.EpayMerchantPrivateKey = "merchant_private_key"
	paymentSetting.EpayPlatformPublicKey = ""
	require.False(t, isEpayWebhookEnabled())

	paymentSetting.EpayPlatformPublicKey = "platform_public_key"
	operation_setting.EpayKey = ""
	require.True(t, isEpayWebhookEnabled())

	// The secondary gateway alone is enough to take Epay payments.
	originalSecondary := [3]string{paymentSetting.SecondaryEpayAddress, paymentSetting.SecondaryEpayId, paymentSetting.SecondaryEpayKey}
	t.Cleanup(func() {
		paymentSetting.SecondaryEpayAddress, paymentSetting.SecondaryEpayId, paymentSetting.SecondaryEpayKey = originalSecondary[0], originalSecondary[1], originalSecondary[2]
	})
	operation_setting.PayAddress = ""
	operation_setting.PayMethods = []map[string]string{{"type": "usdt.trc20", "gateway": operation_setting.EpayGatewaySecondary}}
	paymentSetting.SecondaryEpayAddress = "https://usdt.example.com"
	paymentSetting.SecondaryEpayId = "1000"
	paymentSetting.SecondaryEpayKey = ""
	require.False(t, isEpayWebhookEnabled())

	paymentSetting.SecondaryEpayKey = "secondary-key"
	require.True(t, isEpayWebhookEnabled())
}

// setupTwoEpayGatewaysForTest configures Alipay on the primary gateway and
// USDT on the secondary one, with a user and pending orders on both.
func setupTwoEpayGatewaysForTest(t *testing.T, primaryMode string) (*epay.Client, *epay.Client) {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	originalPayment := *paymentSetting
	t.Cleanup(func() { *paymentSetting = originalPayment })
	original := struct {
		payAddress, epayID, epayKey, serverAddress string
		payMethods                                 []map[string]string
		quotaPerUnit, price                        float64
		minTopUp                                   int
		db, logDB                                  *gorm.DB
	}{
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey, system_setting.ServerAddress,
		operation_setting.PayMethods, common.QuotaPerUnit, operation_setting.Price, operation_setting.MinTopUp, model.DB, model.LOG_DB,
	}
	t.Cleanup(func() {
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = original.payAddress, original.epayID, original.epayKey
		system_setting.ServerAddress = original.serverAddress
		operation_setting.PayMethods = original.payMethods
		common.QuotaPerUnit, operation_setting.Price, operation_setting.MinTopUp = original.quotaPerUnit, original.price, original.minTopUp
		model.DB, model.LOG_DB = original.db, original.logDB
	})
	operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = "https://pay.example.com", "primary_id", "primary-key"
	paymentSetting.SecondaryEpayAddress, paymentSetting.SecondaryEpayId, paymentSetting.SecondaryEpayKey = "https://usdt.example.com", "1000", "secondary-key"
	operation_setting.PayMethods = []map[string]string{
		{"name": "Alipay", "type": "alipay"},
		{"name": "USDT", "type": "usdt.trc20", "gateway": operation_setting.EpayGatewaySecondary},
	}
	system_setting.ServerAddress = "https://dashboard.example.com"
	common.QuotaPerUnit, operation_setting.Price, operation_setting.MinTopUp = 500000, 7.3, 1

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.Log{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}, &model.UserSubscription{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "payer", AffCode: "p1", Group: "default"}).Error)
	require.NoError(t, db.Create(&model.SubscriptionPlan{Id: 1, Title: "Monthly", PriceAmount: 9.9, Currency: "USD", DurationUnit: "month", DurationValue: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&[]model.TopUp{
		{UserId: 1, Amount: 1, Money: 7.3, TradeNo: "ALI1", PaymentMethod: "alipay", PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusPending},
		{UserId: 1, Amount: 2, Money: 14.6, TradeNo: "USDT1", PaymentMethod: "usdt.trc20", PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusPending},
	}).Error)
	require.NoError(t, db.Create(&[]model.SubscriptionOrder{
		{UserId: 1, PlanId: 1, Money: 9.9, TradeNo: "SUBALI1", PaymentMethod: "alipay", PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusPending},
		{UserId: 1, PlanId: 1, Money: 9.9, TradeNo: "SUBUSDT1", PaymentMethod: "usdt.trc20", PaymentProvider: model.PaymentProviderEpay, Status: common.TopUpStatusPending},
	}).Error)

	paymentSetting.EpaySignType = primaryMode
	primaryConfig := &epay.Config{PartnerID: "primary_id", Key: "primary-key", SignType: primaryMode}
	if primaryMode == epay.SignTypeRSA {
		merchantPrivate, merchantPublic := newEpayRSAKeyPairForTest(t)
		platformPrivate, platformPublic := newEpayRSAKeyPairForTest(t)
		paymentSetting.EpayMerchantPrivateKey, paymentSetting.EpayPlatformPublicKey = merchantPrivate, platformPublic
		primaryConfig.MerchantPrivateKey, primaryConfig.PlatformPublicKey = platformPrivate, merchantPublic
	}
	primary, err := epay.NewClient(primaryConfig, "https://pay.example.com")
	require.NoError(t, err)
	secondary, err := epay.NewClient(&epay.Config{PartnerID: "1000", Key: "secondary-key", SignType: epay.SignTypeMD5}, "https://usdt.example.com")
	require.NoError(t, err)
	return primary, secondary
}

func TestEpayPaymentsStartAtThePayMethodGateway(t *testing.T) {
	for _, mode := range []string{epay.SignTypeMD5, epay.SignTypeRSA} {
		t.Run(mode, func(t *testing.T) {
			primary, secondary := setupTwoEpayGatewaysForTest(t, mode)
			primaryURL := "https://pay.example.com/submit.php"
			if mode == epay.SignTypeRSA {
				primaryURL = "https://pay.example.com/api/pay/submit"
			}
			cases := []struct {
				method, wantURL string
				platform        *epay.Client
			}{
				{"alipay", primaryURL, primary},
				{"usdt.trc20", "https://usdt.example.com/submit.php", secondary},
			}
			for _, tc := range cases {
				for _, endpoint := range []struct {
					name    string
					body    string
					handler gin.HandlerFunc
				}{
					{"topup", fmt.Sprintf(`{"amount":1,"payment_method":%q}`, tc.method), RequestEpay},
					{"subscription", fmt.Sprintf(`{"plan_id":1,"payment_method":%q}`, tc.method), SubscriptionRequestEpay},
				} {
					t.Run(endpoint.name+"/"+tc.method, func(t *testing.T) {
						recorder := httptest.NewRecorder()
						ctx, _ := gin.CreateTestContext(recorder)
						ctx.Request = httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(endpoint.body))
						ctx.Request.Header.Set("Content-Type", "application/json")
						ctx.Set("id", 1)

						endpoint.handler(ctx)

						var response struct {
							Message string            `json:"message"`
							URL     string            `json:"url"`
							Data    map[string]string `json:"data"`
						}
						require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response), recorder.Body.String())
						require.Equal(t, "success", response.Message, recorder.Body.String())
						assert.Equal(t, tc.wantURL, response.URL)
						assert.Equal(t, tc.platform.Config.PartnerID, response.Data["pid"])
						assert.Equal(t, tc.platform.Config.SignType, response.Data["sign_type"])
						verified, err := tc.platform.Verify(response.Data)
						require.NoError(t, err)
						assert.True(t, verified.VerifyStatus)
						assert.Equal(t, tc.method, response.Data["type"])
					})
				}
			}
		})
	}
}

type epaySettlementCase struct {
	id, method, money string
	platform, other   *epay.Client
	quota             int
	returnFirst       bool
}

func (order epaySettlementCase) callback(t *testing.T, router *gin.Engine, path string, platform *epay.Client) string {
	t.Helper()
	fields, err := platform.SignParams(map[string]string{
		"out_trade_no": order.id, "trade_no": "GW" + order.id, "money": order.money,
		"name": "item", "type": order.method, "trade_status": epay.StatusTradeSuccess,
	})
	require.NoError(t, err)
	params := url.Values{}
	for k, v := range fields {
		params.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?"+params.Encode(), nil))
	if path == "/api/subscription/epay/return" {
		require.Equal(t, http.StatusFound, rec.Code)
		return rec.Header().Get("Location")
	}
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestEpayCallbacksSettleOnlyThroughTheOrderGateway(t *testing.T) {
	for _, mode := range []string{epay.SignTypeMD5, epay.SignTypeRSA} {
		t.Run(mode, func(t *testing.T) {
			primary, secondary := setupTwoEpayGatewaysForTest(t, mode)
			router := gin.New()
			router.GET("/api/user/epay/notify", EpayNotify)
			router.GET("/api/subscription/epay/notify", SubscriptionEpayNotify)
			router.GET("/api/subscription/epay/return", SubscriptionEpayReturn)
			for _, order := range []epaySettlementCase{
				{"ALI1", "alipay", "7.30", primary, secondary, 500000, false},
				{"USDT1", "usdt.trc20", "14.60", secondary, primary, 1000000, false},
				{"SUBALI1", "alipay", "9.90", primary, secondary, 0, true},
				{"SUBUSDT1", "usdt.trc20", "9.90", secondary, primary, 0, false},
			} {
				t.Run(order.id, func(t *testing.T) {
					if order.quota > 0 {
						const path = "/api/user/epay/notify"
						before, err := model.GetUserById(1, false)
						require.NoError(t, err)
						assert.Equal(t, "fail", order.callback(t, router, path, order.other))
						require.Equal(t, common.TopUpStatusPending, model.GetTopUpByTradeNo(order.id).Status)
						assert.Equal(t, "success", order.callback(t, router, path, order.platform))
						assert.Equal(t, "success", order.callback(t, router, path, order.platform))
						after, err := model.GetUserById(1, false)
						require.NoError(t, err)
						assert.Equal(t, before.Quota+order.quota, after.Quota)
						assert.Equal(t, common.TopUpStatusSuccess, model.GetTopUpByTradeNo(order.id).Status)
						return
					}
					const notifyPath = "/api/subscription/epay/notify"
					const returnPath = "/api/subscription/epay/return"
					const wallet = "https://dashboard.example.com/wallet?pay="
					assert.Equal(t, "fail", order.callback(t, router, notifyPath, order.other))
					assert.Equal(t, wallet+"fail", order.callback(t, router, returnPath, order.other))
					require.Equal(t, common.TopUpStatusPending, model.GetSubscriptionOrderByTradeNo(order.id).Status)
					if order.returnFirst {
						assert.Equal(t, wallet+"success", order.callback(t, router, returnPath, order.platform))
						require.Equal(t, common.TopUpStatusSuccess, model.GetSubscriptionOrderByTradeNo(order.id).Status)
						assert.Equal(t, "success", order.callback(t, router, notifyPath, order.platform))
					} else {
						assert.Equal(t, "success", order.callback(t, router, notifyPath, order.platform))
						require.Equal(t, common.TopUpStatusSuccess, model.GetSubscriptionOrderByTradeNo(order.id).Status)
						assert.Equal(t, wallet+"success", order.callback(t, router, returnPath, order.platform))
					}
				})
			}
			unknown := epaySettlementCase{id: "MISSING", method: "usdt.trc20", money: "14.60"}
			assert.Equal(t, "fail", unknown.callback(t, router, "/api/user/epay/notify", secondary))

			var count int64
			require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", 1).Count(&count).Error)
			assert.EqualValues(t, 2, count)
		})
	}
}

func TestPayMethodsRejectUnknownGateway(t *testing.T) {
	assert.NoError(t, operation_setting.ValidatePayMethodsJSON(`[{"type":"alipay"},{"type":"usdt.trc20","gateway":"secondary"}]`))
	assert.Error(t, operation_setting.ValidatePayMethodsJSON(`[{"type":"usdt.trc20","gateway":"Secondary"}]`))
	assert.Error(t, operation_setting.ValidatePayMethodsJSON(`{"type":"alipay"}`))
}

// epayCallbackCase is one callback sent to every Epay callback endpoint.
type epayCallbackCase struct {
	name         string
	params       map[string]string
	validPending bool
}

// setupEpayCallbackTest configures an Epay gateway and leaves model.DB nil:
// rejected callbacks and valid non-success events must never reach settlement,
// so an accidental attempt to settle fails the test.
func setupEpayCallbackTest(t *testing.T) {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	paymentSetting := operation_setting.GetPaymentSetting()
	originalSignType := paymentSetting.EpaySignType
	originalMerchantPrivateKey := paymentSetting.EpayMerchantPrivateKey
	originalPlatformPublicKey := paymentSetting.EpayPlatformPublicKey
	originalPayMethods := operation_setting.PayMethods
	originalServerAddress := system_setting.ServerAddress
	originalDB := model.DB
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		paymentSetting.EpaySignType = originalSignType
		paymentSetting.EpayMerchantPrivateKey = originalMerchantPrivateKey
		paymentSetting.EpayPlatformPublicKey = originalPlatformPublicKey
		operation_setting.PayMethods = originalPayMethods
		system_setting.ServerAddress = originalServerAddress
		model.DB = originalDB
	})
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	system_setting.ServerAddress = "https://dashboard.example.com"
	model.DB = nil
}

// assertEpayCallbacksVerifyBeforeSettlement sends each callback to the top-up
// and subscription endpoints by GET and POST. Only a valid pending callback is
// accepted; every other callback is rejected.
func assertEpayCallbacksVerifyBeforeSettlement(t *testing.T, cases []epayCallbackCase) {
	t.Helper()
	endpoints := []struct {
		name             string
		path             string
		handler          gin.HandlerFunc
		status           int
		rejectedBody     string
		pendingBody      string
		rejectedLocation string
		pendingLocation  string
	}{
		{
			name: "topup notify", path: "/api/user/epay/notify", handler: EpayNotify,
			status: http.StatusOK, rejectedBody: "fail", pendingBody: "success",
		},
		{
			name: "subscription notify", path: "/api/subscription/epay/notify", handler: SubscriptionEpayNotify,
			status: http.StatusOK, rejectedBody: "fail", pendingBody: "fail",
		},
		{
			name: "subscription return", path: "/api/subscription/epay/return", handler: SubscriptionEpayReturn,
			status:           http.StatusFound,
			rejectedLocation: "https://dashboard.example.com/wallet?pay=fail",
			pendingLocation:  "https://dashboard.example.com/wallet?pay=pending",
		},
	}
	router := gin.New()
	for _, endpoint := range endpoints {
		router.Any(endpoint.path, endpoint.handler)
	}
	for _, endpoint := range endpoints {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			for _, tc := range cases {
				t.Run(endpoint.name+"/"+method+"/"+tc.name, func(t *testing.T) {
					params := url.Values{}
					for key, value := range tc.params {
						params.Set(key, value)
					}
					request := httptest.NewRequest(method, endpoint.path+"?"+params.Encode(), nil)
					if method == http.MethodPost {
						request = httptest.NewRequest(method, endpoint.path, strings.NewReader(params.Encode()))
						request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					}
					recorder := httptest.NewRecorder()
					require.NotPanics(t, func() { router.ServeHTTP(recorder, request) })
					assert.Equal(t, endpoint.status, recorder.Code)
					wantBody, wantLocation := endpoint.rejectedBody, endpoint.rejectedLocation
					if tc.validPending {
						wantBody, wantLocation = endpoint.pendingBody, endpoint.pendingLocation
					}
					if wantBody != "" {
						assert.Equal(t, wantBody, recorder.Body.String())
					}
					assert.Equal(t, wantLocation, recorder.Header().Get("Location"))
				})
			}
		}
	}
}

// newEpayRSAKeyPairForTest returns a throwaway key pair as single-line Base64
// text, the format Epay merchant consoles show.
func newEpayRSAKeyPairForTest(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(privateDER), base64.StdEncoding.EncodeToString(publicDER)
}

func TestEpayCallbacksVerifyBeforeSettlement(t *testing.T) {
	setupEpayCallbackTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeMD5
	operation_setting.EpayKey = "test-key"
	require.True(t, isEpayWebhookEnabled())

	// Both ambiguous maps below had this same valid signing string in v0.0.4.
	// This must fail because of parameter ambiguity, not a mismatched signature.
	ambiguousSign := epay.MD5String("money=1.00&name=item&out_trade_no=other&out_trade_no=order&pid=epay_id&trade_status=TRADE_SUCCESS&type=alipay", "test-key")
	pendingSign := epay.MD5String("money=1.00&name=item%26other&out_trade_no=order&pid=epay_id&trade_status=WAIT_BUYER_PAY&type=alipay", "test-key")
	rows := []struct {
		name         string
		productName  string
		tradeNo      string
		status       string
		sign         string
		validPending bool
	}{
		{"separator in product name", "item&out_trade_no=other", "order", epay.StatusTradeSuccess, ambiguousSign, false},
		{"separator in order number", "item", "other&out_trade_no=order", epay.StatusTradeSuccess, ambiguousSign, false},
		{"invalid signature", "item", "order", epay.StatusTradeSuccess, "invalid", false},
		{"missing signature", "item", "order", epay.StatusTradeSuccess, "", false},
		{"valid pending with literal percent escape", "item%26other", "order", "WAIT_BUYER_PAY", pendingSign, true},
	}
	cases := make([]epayCallbackCase, 0, len(rows))
	for _, row := range rows {
		params := map[string]string{
			"pid": "epay_id", "type": "alipay", "money": "1.00",
			"name": row.productName, "out_trade_no": row.tradeNo,
			"trade_status": row.status, "sign_type": "MD5",
		}
		if row.sign != "" {
			params["sign"] = row.sign
		}
		cases = append(cases, epayCallbackCase{name: row.name, params: params, validPending: row.validPending})
	}
	legacy, err := epay.NewClient(&epay.Config{PartnerID: "epay_id", Key: "test-key"}, operation_setting.PayAddress)
	require.NoError(t, err)
	pending, err := legacy.SignParams(map[string]string{
		"type": "alipay", "money": "1.00", "name": "item", "out_trade_no": "order", "trade_status": "WAIT_BUYER_PAY",
	})
	require.NoError(t, err)
	delete(pending, "sign_type")
	wrongLabel := maps.Clone(pending)
	wrongLabel["sign_type"] = epay.SignTypeRSA
	cases = append(cases,
		epayCallbackCase{name: "legacy MD5 without sign type", params: pending, validPending: true},
		epayCallbackCase{name: "MD5 with conflicting sign type", params: wrongLabel},
	)
	assertEpayCallbacksVerifyBeforeSettlement(t, cases)
}

func TestEpayRSAModeSignsOrdersAndVerifiesCallbacks(t *testing.T) {
	setupEpayCallbackTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	merchantPrivateKey, merchantPublicKey := newEpayRSAKeyPairForTest(t)
	platformPrivateKey, platformPublicKey := newEpayRSAKeyPairForTest(t)
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeRSA
	// A leftover MD5 key must not be accepted once RSA is selected.
	operation_setting.EpayKey = "test-key"
	paymentSetting.EpayMerchantPrivateKey = merchantPrivateKey
	paymentSetting.EpayPlatformPublicKey = platformPublicKey
	require.True(t, isEpayWebhookEnabled())

	client := GetEpayClient()
	require.NotNil(t, client)
	notifyURL, err := url.Parse("https://dashboard.example.com/api/user/epay/notify")
	require.NoError(t, err)
	returnURL, err := url.Parse("https://dashboard.example.com/usage-logs")
	require.NoError(t, err)
	paymentURL, order, err := client.Purchase(&epay.PurchaseArgs{
		Type: "alipay", ServiceTradeNo: "order", Name: "TUC1", Money: "1.00",
		Device: epay.PC, NotifyUrl: notifyURL, ReturnUrl: returnURL,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://pay.example.com/api/pay/submit", paymentURL)
	assert.Equal(t, epay.SignTypeRSA, order["sign_type"])

	// The platform side holds the merchant public key and signs callbacks with its own private key.
	platform, err := epay.NewClient(&epay.Config{
		PartnerID: "epay_id", SignType: epay.SignTypeRSA,
		MerchantPrivateKey: platformPrivateKey, PlatformPublicKey: merchantPublicKey,
	}, "https://pay.example.com")
	require.NoError(t, err)
	verifiedOrder, err := platform.Verify(order)
	require.NoError(t, err)
	assert.True(t, verifiedOrder.VerifyStatus)

	pending, err := platform.SignParams(map[string]string{
		"type": "alipay", "money": "1.00", "name": "item", "out_trade_no": "order", "trade_status": "WAIT_BUYER_PAY",
	})
	require.NoError(t, err)
	paidFields := map[string]string{
		"type": "alipay", "money": "1.00", "name": "item", "out_trade_no": "order", "trade_status": epay.StatusTradeSuccess,
	}
	paid, err := platform.SignParams(paidFields)
	require.NoError(t, err)
	tampered := maps.Clone(paid)
	tampered["money"] = "100.00"
	merchantSigned, err := client.SignParams(paidFields)
	require.NoError(t, err)
	md5Client, err := epay.NewClient(&epay.Config{PartnerID: "epay_id", Key: "test-key"}, "https://pay.example.com")
	require.NoError(t, err)
	md5Signed, err := md5Client.SignParams(paidFields)
	require.NoError(t, err)
	unsigned := maps.Clone(paid)
	delete(unsigned, "sign")
	otherMerchant := *platform.Config
	otherMerchant.PartnerID = "other-merchant"
	other, err := epay.NewClient(&otherMerchant, "https://pay.example.com")
	require.NoError(t, err)
	otherPaid, err := other.SignParams(paidFields)
	require.NoError(t, err)

	assertEpayCallbacksVerifyBeforeSettlement(t, []epayCallbackCase{
		{name: "valid pending signed by platform", params: pending, validPending: true},
		{name: "same platform but another merchant", params: otherPaid},
		{name: "tampered amount", params: tampered},
		{name: "signed with merchant key", params: merchantSigned},
		{name: "MD5 signature in RSA mode", params: md5Signed},
		{name: "missing signature", params: unsigned},
	})
}

func TestUpdateOptionValidatesEpaySignSettings(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	originalOptionMap := common.OptionMap
	paymentSetting := operation_setting.GetPaymentSetting()
	originalSignType := paymentSetting.EpaySignType
	originalMerchantPrivateKey := paymentSetting.EpayMerchantPrivateKey
	originalPlatformPublicKey := paymentSetting.EpayPlatformPublicKey
	t.Cleanup(func() {
		common.OptionMap = originalOptionMap
		paymentSetting.EpaySignType = originalSignType
		paymentSetting.EpayMerchantPrivateKey = originalMerchantPrivateKey
		paymentSetting.EpayPlatformPublicKey = originalPlatformPublicKey
	})
	// A site upgraded from an MD5-only release has no saved RSA settings.
	common.OptionMap = map[string]string{
		operation_setting.EpaySignTypeOptionKey:           operation_setting.EpaySignTypeMD5,
		operation_setting.EpayMerchantPrivateKeyOptionKey: "",
		operation_setting.EpayPlatformPublicKeyOptionKey:  "",
	}
	paymentSetting.EpaySignType = operation_setting.EpaySignTypeMD5
	paymentSetting.EpayMerchantPrivateKey = ""
	paymentSetting.EpayPlatformPublicKey = ""
	signTypeKey := operation_setting.EpaySignTypeOptionKey
	privateKeyKey := operation_setting.EpayMerchantPrivateKeyOptionKey
	publicKeyKey := operation_setting.EpayPlatformPublicKeyOptionKey
	merchantPrivateKey, _ := newEpayRSAKeyPairForTest(t)
	_, platformPublicKey := newEpayRSAKeyPairForTest(t)
	const privateKeyError = "The Epay merchant private key is missing or invalid"
	const publicKeyError = "The Epay platform public key is missing or invalid"

	steps := []struct {
		name      string
		key       string
		value     string
		wantError string
	}{
		{"unknown signature type", signTypeKey, "SHA1", "Epay signature type must be MD5 or RSA"},
		{"RSA before keys are saved", signTypeKey, "RSA", privateKeyError},
		{"malformed private key", privateKeyKey, "not a key", privateKeyError},
		{"public key in the private key field", privateKeyKey, platformPublicKey, privateKeyError},
		{"save private key", privateKeyKey, merchantPrivateKey, ""},
		{"RSA before the public key is saved", signTypeKey, "RSA", publicKeyError},
		{"private key in the public key field", publicKeyKey, merchantPrivateKey, publicKeyError},
		{"save public key", publicKeyKey, platformPublicKey, ""},
		{"switch to RSA", signTypeKey, "RSA", ""},
		{"clear private key while RSA is active", privateKeyKey, "", privateKeyError},
		{"switch back to MD5", signTypeKey, "MD5", ""},
	}
	for _, step := range steps {
		err := model.UpdateOption(step.key, step.value)
		if step.wantError == "" {
			require.NoError(t, err, step.name)
			continue
		}
		require.Error(t, err, step.name)
		assert.Equal(t, step.wantError, err.Error(), step.name)
	}
	assert.Equal(t, operation_setting.EpaySignTypeMD5, paymentSetting.EpaySignType)
	assert.Equal(t, merchantPrivateKey, paymentSetting.EpayMerchantPrivateKey)
	assert.Equal(t, platformPublicKey, paymentSetting.EpayPlatformPublicKey)

	// The private key is never sent back to the settings page; the public key may be.
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)
	GetOptions(context)
	var response struct {
		Success bool           `json:"success"`
		Data    []model.Option `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	returned := make(map[string]string, len(response.Data))
	for _, option := range response.Data {
		returned[option.Key] = option.Value
	}
	assert.Equal(t, operation_setting.EpaySignTypeMD5, returned[signTypeKey])
	assert.Equal(t, platformPublicKey, returned[publicKeyKey])
	assert.NotContains(t, returned, privateKeyKey)

	// An unsuccessful settings write must not switch the live gateway to RSA.
	require.NoError(t, db.Migrator().DropTable(&model.Option{}))
	require.Error(t, model.UpdateOption(signTypeKey, operation_setting.EpaySignTypeRSA))
	assert.Equal(t, operation_setting.EpaySignTypeMD5, paymentSetting.EpaySignType)
}
