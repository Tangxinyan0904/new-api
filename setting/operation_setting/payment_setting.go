package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type PaymentSetting struct {
	AmountOptions  []int           `json:"amount_options"`
	AmountDiscount map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠

	ComplianceConfirmed    bool   `json:"compliance_confirmed"`
	ComplianceTermsVersion string `json:"compliance_terms_version"`
	ComplianceConfirmedAt  int64  `json:"compliance_confirmed_at"`
	ComplianceConfirmedBy  int    `json:"compliance_confirmed_by"`
	ComplianceConfirmedIP  string `json:"compliance_confirmed_ip"`

	// A second Epay-compatible gateway (for example a crypto gateway) for the
	// pay methods whose "gateway" is EpayGatewaySecondary.
	SecondaryEpayAddress string `json:"secondary_epay_address"`
	SecondaryEpayId      string `json:"secondary_epay_id"`
	SecondaryEpayKey     string `json:"secondary_epay_key"`
}

const CurrentComplianceTermsVersion = "v1"

// EpayGatewayPrimary and EpayGatewaySecondary are the values of a pay
// method's "gateway" field. The primary gateway is PayAddress/EpayId/EpayKey.
const (
	EpayGatewayPrimary   = ""
	EpayGatewaySecondary = "secondary"
)

// EpayGatewayOf returns the Epay gateway that serves a pay method type.
// Types missing from PayMethods belong to the primary gateway.
func EpayGatewayOf(payMethod string) string {
	for _, method := range PayMethods {
		if method["type"] == payMethod {
			return method["gateway"]
		}
	}
	return EpayGatewayPrimary
}

// ValidatePayMethodsJSON rejects pay methods that name an unknown gateway,
// so a typo cannot send orders to the wrong merchant.
func ValidatePayMethodsJSON(value string) error {
	var methods []map[string]string
	if err := common.UnmarshalJsonStr(value, &methods); err != nil {
		return common.NewMessage("Payment methods must be a JSON array")
	}
	for _, method := range methods {
		gateway := method["gateway"]
		if gateway != EpayGatewayPrimary && gateway != EpayGatewaySecondary {
			return common.NewMessage("Payment method gateway must be empty or secondary")
		}
	}
	return nil
}

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:  []int{10, 20, 50, 100, 200, 500},
	AmountDiscount: map[int]float64{},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

func IsPaymentComplianceConfirmed() bool {
	return paymentSetting.ComplianceConfirmed &&
		paymentSetting.ComplianceTermsVersion == CurrentComplianceTermsVersion
}
