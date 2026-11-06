package controller

import (
	"slices"
	"strings"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// epayGateways lists every Epay-compatible gateway a pay method can use.
var epayGateways = []string{operation_setting.EpayGatewayPrimary, operation_setting.EpayGatewaySecondary}

// epayClientForGateway returns the client of one Epay gateway, or nil when
// that gateway is not configured.
func epayClientForGateway(gateway string) *epay.Client {
	if gateway != operation_setting.EpayGatewaySecondary {
		return GetEpayClient()
	}
	if !isEpayGatewayConfigured(gateway) {
		return nil
	}
	paymentSetting := operation_setting.GetPaymentSetting()
	client, err := epay.NewClient(&epay.Config{
		PartnerID: strings.TrimSpace(paymentSetting.SecondaryEpayId),
		Key:       strings.TrimSpace(paymentSetting.SecondaryEpayKey),
	}, strings.TrimSpace(paymentSetting.SecondaryEpayAddress))
	if err != nil {
		return nil
	}
	return client
}

func isEpayGatewayConfigured(gateway string) bool {
	if gateway != operation_setting.EpayGatewaySecondary {
		return strings.TrimSpace(operation_setting.PayAddress) != "" &&
			strings.TrimSpace(operation_setting.EpayId) != "" &&
			strings.TrimSpace(operation_setting.EpayKey) != ""
	}
	paymentSetting := operation_setting.GetPaymentSetting()
	return strings.TrimSpace(paymentSetting.SecondaryEpayAddress) != "" &&
		strings.TrimSpace(paymentSetting.SecondaryEpayId) != "" &&
		strings.TrimSpace(paymentSetting.SecondaryEpayKey) != ""
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
		result, err := client.Verify(params)
		if err != nil || !result.VerifyStatus {
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
