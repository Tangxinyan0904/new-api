package geoip_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBlockedCountriesJSON(t *testing.T) {
	got, err := NormalizeBlockedCountriesJSON(`["cn"," US ","cn"]`)

	require.NoError(t, err)
	require.JSONEq(t, `["CN","US"]`, got)
}

func TestNormalizeBlockedCountriesJSONRejectsInvalidCode(t *testing.T) {
	_, err := NormalizeBlockedCountriesJSON(`["china"]`)

	require.Error(t, err)
}

func TestNormalizeBlockedCountriesJSONRejectsNonArray(t *testing.T) {
	_, err := NormalizeBlockedCountriesJSON(`{"code":"CN"}`)

	require.Error(t, err)
}

func TestValidateModeRejectsUnknownMode(t *testing.T) {
	require.NoError(t, ValidateMode(ModeOff))
	require.NoError(t, ValidateMode(ModeHomepageNotice))
	require.NoError(t, ValidateMode(ModeHomepageBlock))
	require.NoError(t, ValidateMode(ModeHomepageBlockAPIReject))
	require.NoError(t, ValidateMode(ModeFullReject))

	require.Error(t, ValidateMode("unknown"))
}
