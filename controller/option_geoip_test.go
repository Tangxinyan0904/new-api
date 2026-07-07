package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/geoip_setting"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGeoIPOptionValueNormalizesBlockedCountries(t *testing.T) {
	got, err := normalizeGeoIPOptionValue("geoip.blocked_countries", `["cn"," US ","cn"]`)

	require.NoError(t, err)
	require.JSONEq(t, `["CN","US"]`, got)
}

func TestNormalizeGeoIPOptionValueRejectsInvalidMode(t *testing.T) {
	_, err := normalizeGeoIPOptionValue("geoip.mode", "unknown")

	require.Error(t, err)
}

func TestNormalizeGeoIPOptionValueAcceptsValidMode(t *testing.T) {
	got, err := normalizeGeoIPOptionValue("geoip.mode", geoip_setting.ModeFullReject)

	require.NoError(t, err)
	require.Equal(t, geoip_setting.ModeFullReject, got)
}

func TestIsSensitiveOptionKeyHidesLowercaseGeoIPKey(t *testing.T) {
	require.True(t, isSensitiveOptionKey("geoip.maxmind_license_key"))
	require.True(t, isSensitiveOptionKey("StripeApiSecret"))
	require.False(t, isSensitiveOptionKey("geoip.download_url"))
}
