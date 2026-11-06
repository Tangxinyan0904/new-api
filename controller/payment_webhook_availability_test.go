package controller

import (
	"fmt"
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
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = ""
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	require.False(t, isEpayWebhookEnabled())

	operation_setting.EpayKey = "epay_key"
	require.True(t, isEpayWebhookEnabled())

	operation_setting.PayMethods = nil
	require.False(t, isEpayWebhookEnabled())

	// The secondary gateway alone is enough to take Epay payments.
	paymentSetting := operation_setting.GetPaymentSetting()
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
func setupTwoEpayGatewaysForTest(t *testing.T) {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	original := struct {
		payAddress, epayID, epayKey, serverAddress string
		secondary                                  [3]string
		payMethods                                 []map[string]string
		quotaPerUnit, price                        float64
		minTopUp                                   int
		db, logDB                                  *gorm.DB
	}{
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey, system_setting.ServerAddress,
		[3]string{paymentSetting.SecondaryEpayAddress, paymentSetting.SecondaryEpayId, paymentSetting.SecondaryEpayKey},
		operation_setting.PayMethods, common.QuotaPerUnit, operation_setting.Price, operation_setting.MinTopUp, model.DB, model.LOG_DB,
	}
	t.Cleanup(func() {
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = original.payAddress, original.epayID, original.epayKey
		system_setting.ServerAddress = original.serverAddress
		paymentSetting.SecondaryEpayAddress, paymentSetting.SecondaryEpayId, paymentSetting.SecondaryEpayKey = original.secondary[0], original.secondary[1], original.secondary[2]
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
}

func TestEpayPaymentsStartAtThePayMethodGateway(t *testing.T) {
	setupTwoEpayGatewaysForTest(t)
	cases := []struct {
		method, wantURL, wantPID string
	}{
		{"alipay", "https://pay.example.com/submit.php", "primary_id"},
		{"usdt.trc20", "https://usdt.example.com/submit.php", "1000"},
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
				assert.Equal(t, tc.wantPID, response.Data["pid"])
				assert.Equal(t, tc.method, response.Data["type"])
			})
		}
	}
}

func TestEpayCallbacksSettleOnlyThroughTheOrderGateway(t *testing.T) {
	setupTwoEpayGatewaysForTest(t)
	signedCallback := func(tradeNo, method, pid, key string) url.Values {
		params := url.Values{
			"money": {"1.00"}, "name": {"item"}, "out_trade_no": {tradeNo}, "pid": {pid},
			"trade_no": {"GW" + tradeNo}, "trade_status": {epay.StatusTradeSuccess}, "type": {method},
		}
		params.Set("sign", epay.MD5String(fmt.Sprintf("money=1.00&name=item&out_trade_no=%s&pid=%s&trade_no=GW%s&trade_status=%s&type=%s",
			tradeNo, pid, tradeNo, epay.StatusTradeSuccess, method), key))
		params.Set("sign_type", "MD5")
		return params
	}
	router := gin.New()
	router.GET("/api/user/epay/notify", EpayNotify)
	router.GET("/api/subscription/epay/notify", SubscriptionEpayNotify)
	call := func(path string, params url.Values) string {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path+"?"+params.Encode(), nil))
		return recorder.Body.String()
	}
	quota := func() int {
		user, err := model.GetUserById(1, true)
		require.NoError(t, err)
		return user.Quota
	}

	// A key of one gateway cannot settle the other gateway's orders.
	assert.Equal(t, "fail", call("/api/user/epay/notify", signedCallback("USDT1", "usdt.trc20", "primary_id", "primary-key")))
	assert.Equal(t, "fail", call("/api/user/epay/notify", signedCallback("ALI1", "alipay", "1000", "secondary-key")))
	assert.Equal(t, "fail", call("/api/subscription/epay/notify", signedCallback("SUBUSDT1", "usdt.trc20", "primary_id", "primary-key")))
	assert.Equal(t, "fail", call("/api/subscription/epay/notify", signedCallback("SUBALI1", "alipay", "1000", "secondary-key")))
	assert.Equal(t, common.TopUpStatusPending, model.GetTopUpByTradeNo("USDT1").Status)
	assert.Equal(t, common.TopUpStatusPending, model.GetTopUpByTradeNo("ALI1").Status)
	assert.Equal(t, common.TopUpStatusPending, model.GetSubscriptionOrderByTradeNo("SUBUSDT1").Status)
	assert.Equal(t, common.TopUpStatusPending, model.GetSubscriptionOrderByTradeNo("SUBALI1").Status)
	assert.Zero(t, quota())

	// Each gateway settles its own orders.
	assert.Equal(t, "success", call("/api/user/epay/notify", signedCallback("USDT1", "usdt.trc20", "1000", "secondary-key")))
	assert.Equal(t, 2*500000, quota())
	assert.Equal(t, "success", call("/api/user/epay/notify", signedCallback("ALI1", "alipay", "primary_id", "primary-key")))
	assert.Equal(t, 3*500000, quota())
	assert.Equal(t, "success", call("/api/subscription/epay/notify", signedCallback("SUBUSDT1", "usdt.trc20", "1000", "secondary-key")))
	assert.Equal(t, common.TopUpStatusSuccess, model.GetSubscriptionOrderByTradeNo("SUBUSDT1").Status)
	assert.Equal(t, "success", call("/api/subscription/epay/notify", signedCallback("SUBALI1", "alipay", "primary_id", "primary-key")))
	assert.Equal(t, common.TopUpStatusSuccess, model.GetSubscriptionOrderByTradeNo("SUBALI1").Status)

	// A signed callback for an unknown order fails closed.
	assert.Equal(t, "fail", call("/api/user/epay/notify", signedCallback("MISSING", "usdt.trc20", "1000", "secondary-key")))
}

func TestPayMethodsRejectUnknownGateway(t *testing.T) {
	assert.NoError(t, operation_setting.ValidatePayMethodsJSON(`[{"type":"alipay"},{"type":"usdt.trc20","gateway":"secondary"}]`))
	assert.Error(t, operation_setting.ValidatePayMethodsJSON(`[{"type":"usdt.trc20","gateway":"Secondary"}]`))
	assert.Error(t, operation_setting.ValidatePayMethodsJSON(`{"type":"alipay"}`))
}

func TestEpayCallbacksVerifyBeforeSettlement(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	originalServerAddress := system_setting.ServerAddress
	originalDB := model.DB
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
		system_setting.ServerAddress = originalServerAddress
		model.DB = originalDB
	})
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = "test-key"
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	system_setting.ServerAddress = "https://dashboard.example.com"
	// Rejected callbacks and valid non-success events must never reach settlement.
	// A nil database makes an accidental attempt to settle fail the test.
	model.DB = nil
	require.True(t, isEpayWebhookEnabled())

	// Both ambiguous maps below had this same valid signing string in v0.0.4.
	// This must fail because of parameter ambiguity, not a mismatched signature.
	ambiguousSign := epay.MD5String("money=1.00&name=item&out_trade_no=other&out_trade_no=order&pid=epay_id&trade_status=TRADE_SUCCESS&type=alipay", "test-key")
	pendingSign := epay.MD5String("money=1.00&name=item%26other&out_trade_no=order&pid=epay_id&trade_status=WAIT_BUYER_PAY&type=alipay", "test-key")
	cases := []struct {
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
					params := url.Values{
						"pid": {"epay_id"}, "type": {"alipay"}, "money": {"1.00"},
						"name": {tc.productName}, "out_trade_no": {tc.tradeNo},
						"trade_status": {tc.status}, "sign_type": {"MD5"},
					}
					if tc.sign != "" {
						params.Set("sign", tc.sign)
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
