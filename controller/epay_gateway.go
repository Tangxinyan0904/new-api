package controller

import (
	"net/http"
	"slices"
	"strings"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// epayGateways lists every Epay-compatible gateway a pay method can use.
var epayGateways = []string{operation_setting.EpayGatewayPrimary, operation_setting.EpayGatewaySecondary}

// epayClientForGateway returns the client of one Epay gateway, or nil when
// that gateway is not configured.
func epayClientForGateway(gateway string) *epay.Client {
	config, address := epayGatewayConfig(gateway)
	if config == nil {
		return nil
	}
	client, err := epay.NewClient(config, address)
	if err != nil {
		return nil
	}
	return client
}

// epayGatewayConfig snapshots one gateway's credentials and signing mode together.
func epayGatewayConfig(gateway string) (*epay.Config, string) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	paymentSetting := operation_setting.GetPaymentSetting()
	var config epay.Config
	var address string
	switch gateway {
	case operation_setting.EpayGatewayPrimary:
		address = operation_setting.PayAddress
		config.PartnerID = operation_setting.EpayId
		switch paymentSetting.EpaySignType {
		case "", operation_setting.EpaySignTypeMD5:
			config.SignType, config.Key = epay.SignTypeMD5, operation_setting.EpayKey
		case operation_setting.EpaySignTypeRSA:
			config.SignType = epay.SignTypeRSA
			config.MerchantPrivateKey = paymentSetting.EpayMerchantPrivateKey
			config.PlatformPublicKey = paymentSetting.EpayPlatformPublicKey
			if strings.TrimSpace(config.MerchantPrivateKey) == "" || strings.TrimSpace(config.PlatformPublicKey) == "" {
				return nil, ""
			}
		default:
			return nil, ""
		}
	case operation_setting.EpayGatewaySecondary:
		address = strings.TrimSpace(paymentSetting.SecondaryEpayAddress)
		config.PartnerID = strings.TrimSpace(paymentSetting.SecondaryEpayId)
		config.SignType, config.Key = epay.SignTypeMD5, strings.TrimSpace(paymentSetting.SecondaryEpayKey)
	default:
		return nil, ""
	}
	if strings.TrimSpace(address) == "" || strings.TrimSpace(config.PartnerID) == "" {
		return nil, ""
	}
	if config.SignType == epay.SignTypeMD5 && strings.TrimSpace(config.Key) == "" {
		return nil, ""
	}
	return &config, address
}

// GetEpayClient returns the primary Epay client.
func GetEpayClient() *epay.Client {
	return epayClientForGateway(operation_setting.EpayGatewayPrimary)
}

// epayCallbackParams reads exactly one value per field from the gateway's
// query (GET) or body (POST). Never mix a body signature with query fields.
func epayCallbackParams(r *http.Request) map[string]string {
	if err := r.ParseForm(); err != nil {
		return nil
	}
	fields := r.Form
	if r.Method == http.MethodPost {
		fields = r.PostForm
	}
	params := make(map[string]string, len(fields))
	seen := make(map[string]bool, len(fields))
	for key, values := range fields {
		// The SDK decodes callback field names case-insensitively.
		name := strings.ToLower(key)
		if len(values) != 1 || seen[name] {
			return nil
		}
		seen[name] = true
		params[key] = values[0]
	}
	return params
}

// verifyEpayCallback checks a callback signature against every configured
// gateway. It returns the decoded callback and the gateways whose key signed
// it; no gateway means the callback is forged. It never reads the database,
// so rejected callbacks cost nothing.
func verifyEpayCallback(params map[string]string) (*epay.VerifyRes, []string) {
	var verified *epay.VerifyRes
	var signedBy []string
	for _, gateway := range epayGateways {
		client := epayClientForGateway(gateway)
		if client == nil {
			continue
		}
		// The configured client chooses the algorithm. Some legacy callbacks
		// omit the label; an explicit label must agree with this gateway.
		if method := params["sign_type"]; method != "" && !strings.EqualFold(method, client.Config.SignType) {
			continue
		}
		result, err := client.Verify(params)
		if err != nil || !result.VerifyStatus {
			continue
		}
		// An RSA platform can sign for multiple merchants with the same key.
		if client.Config.SignType == epay.SignTypeRSA && params["pid"] != client.Config.PartnerID {
			continue
		}
		verified = result
		signedBy = append(signedBy, gateway)
	}
	return verified, signedBy
}

// epayCallbackFromOrderGateway reports whether a verified callback was signed
// by the gateway that serves the order's pay method. Without this check a
// leaked key of one gateway could settle the orders of the other.
func epayCallbackFromOrderGateway(orderPayMethod string, signedBy []string) bool {
	return slices.Contains(signedBy, operation_setting.EpayGatewayOf(orderPayMethod))
}
