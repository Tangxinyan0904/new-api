package geoip_setting

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	ModeOff                    = "off"
	ModeHomepageNotice         = "homepage_notice"
	ModeHomepageBlock          = "homepage_block"
	ModeHomepageBlockAPIReject = "homepage_block_api_reject"
	ModeFullReject             = "full_reject"
)

const DefaultPopupMessage = "Your current region is not supported by this service. Please contact the administrator if you believe this is a mistake."

var (
	Mode                 = ModeOff
	DatabasePath         = "Country.mmdb"
	DownloadURL          = ""
	MaxMindLicenseKey    = ""
	PopupMessage         = DefaultPopupMessage
	AllowPrivateLoopback = true
	BlockedCountries     = []string{"CN"}
)

var alpha2CountryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)

func ValidateMode(mode string) error {
	switch mode {
	case ModeOff, ModeHomepageNotice, ModeHomepageBlock, ModeHomepageBlockAPIReject, ModeFullReject:
		return nil
	default:
		return fmt.Errorf("invalid GeoIP access mode: %s", mode)
	}
}

func NormalizeBlockedCountriesJSON(raw string) (string, error) {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return "", fmt.Errorf("invalid GeoIP blocked countries JSON: %w", err)
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		code := strings.ToUpper(strings.TrimSpace(value))
		if !alpha2CountryCodePattern.MatchString(code) {
			return "", fmt.Errorf("invalid ISO 3166-1 alpha-2 country code: %s", value)
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	sort.Strings(normalized)

	bytes, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func UpdateBlockedCountriesByJSONString(raw string) error {
	normalized, err := NormalizeBlockedCountriesJSON(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(normalized), &BlockedCountries)
}

func BlockedCountries2JSONString() string {
	bytes, err := json.Marshal(BlockedCountries)
	if err != nil {
		return `["CN"]`
	}
	return string(bytes)
}
