package setting

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitUser = map[int][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex

type DistillationRateLimitSettings struct {
	Enabled            bool
	Threshold          int
	RPM                int
	PenaltyMinutes     int
	ObservationMinutes int
}

type ModelRequestRateLimitSettings struct {
	Enabled         bool
	DurationMinutes int
	Count           int
	SuccessCount    int
	GroupJSON       string
	UserJSON        string
	Distillation    DistillationRateLimitSettings
}

var modelRequestRateLimitDistillationSettings DistillationRateLimitSettings

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := common.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model request group rate limits: " + err.Error())
		return "{}"
	}
	return string(jsonBytes)
}

func ModelRequestRateLimitUser2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := common.Marshal(ModelRequestRateLimitUser)
	if err != nil {
		common.SysLog("error marshalling model request user rate limits: " + err.Error())
		return "{}"
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	groupLimits, err := parseModelRequestRateLimitGroup(jsonStr)
	if err != nil {
		return err
	}

	ModelRequestRateLimitMutex.Lock()
	ModelRequestRateLimitGroup = groupLimits
	ModelRequestRateLimitMutex.Unlock()
	return nil
}

func UpdateModelRequestRateLimitUserByJSONString(jsonStr string) error {
	userLimits, err := parseModelRequestRateLimitUser(jsonStr)
	if err != nil {
		return err
	}

	ModelRequestRateLimitMutex.Lock()
	ModelRequestRateLimitUser = userLimits
	ModelRequestRateLimitMutex.Unlock()
	return nil
}

func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func GetUserRateLimit(userID int) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	limits, found := ModelRequestRateLimitUser[userID]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func ResolveModelRequestRateLimit(
	userID int,
	group string,
	globalTotal int,
	globalSuccess int,
) (totalCount, successCount int) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	totalCount = globalTotal
	successCount = globalSuccess
	if limits, found := ModelRequestRateLimitGroup[group]; found {
		totalCount = limits[0]
		successCount = limits[1]
	}
	if limits, found := ModelRequestRateLimitUser[userID]; found {
		totalCount = limits[0]
		successCount = limits[1]
	}
	return totalCount, successCount
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	_, err := parseModelRequestRateLimitGroup(jsonStr)
	return err
}

func CheckModelRequestRateLimitUser(jsonStr string) error {
	_, err := parseModelRequestRateLimitUser(jsonStr)
	return err
}

func GetDistillationRateLimitSettings() DistillationRateLimitSettings {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()
	return modelRequestRateLimitDistillationSettings
}

func ValidateDistillationRateLimitSettings(settings DistillationRateLimitSettings) error {
	values := []struct {
		name  string
		value int
	}{
		{name: "threshold", value: settings.Threshold},
		{name: "RPM", value: settings.RPM},
		{name: "penalty minutes", value: settings.PenaltyMinutes},
		{name: "observation minutes", value: settings.ObservationMinutes},
	}
	for _, item := range values {
		if item.value < 0 || item.value > math.MaxInt32 {
			return fmt.Errorf("distillation %s must be between 0 and %d", item.name, math.MaxInt32)
		}
	}
	if !settings.Enabled {
		return nil
	}
	for _, item := range values {
		if item.value == 0 {
			return fmt.Errorf("distillation %s must be positive when detection is enabled", item.name)
		}
	}
	if settings.RPM >= settings.Threshold {
		return fmt.Errorf("distillation punishment RPM must be lower than the detection threshold")
	}
	return nil
}

func ValidateModelRequestRateLimitSettings(settings ModelRequestRateLimitSettings) error {
	if settings.DurationMinutes < 0 || settings.DurationMinutes > math.MaxInt32 {
		return fmt.Errorf("rate limit duration must be between 0 and %d", math.MaxInt32)
	}
	if settings.Count < 0 || settings.Count > math.MaxInt32 {
		return fmt.Errorf("total request limit must be between 0 and %d", math.MaxInt32)
	}
	if settings.SuccessCount < 1 || settings.SuccessCount > math.MaxInt32 {
		return fmt.Errorf("successful request limit must be between 1 and %d", math.MaxInt32)
	}
	if err := CheckModelRequestRateLimitGroup(settings.GroupJSON); err != nil {
		return err
	}
	if err := CheckModelRequestRateLimitUser(settings.UserJSON); err != nil {
		return err
	}
	return ValidateDistillationRateLimitSettings(settings.Distillation)
}

func ValidateModelRequestRateLimitOption(key string, value string) error {
	switch key {
	case "ModelRequestRateLimitGroup":
		return CheckModelRequestRateLimitGroup(value)
	case "ModelRequestRateLimitUser":
		return CheckModelRequestRateLimitUser(value)
	case "ModelRequestRateLimitCount":
		return validateRateLimitInteger(value, 0, "total request limit")
	case "ModelRequestRateLimitDurationMinutes":
		return validateRateLimitInteger(value, 0, "rate limit duration")
	case "ModelRequestRateLimitSuccessCount":
		return validateRateLimitInteger(value, 1, "successful request limit")
	case "ModelRequestRateLimitDistillationEnabled",
		"ModelRequestRateLimitDistillationThreshold",
		"ModelRequestRateLimitDistillationRPM",
		"ModelRequestRateLimitDistillationPenaltyMinutes",
		"ModelRequestRateLimitDistillationObservationMinutes":
		candidate := GetDistillationRateLimitSettings()
		if err := applyDistillationRateLimitOption(&candidate, key, value); err != nil {
			return err
		}
		return ValidateDistillationRateLimitSettings(candidate)
	default:
		return nil
	}
}

func UpdateDistillationRateLimitOption(key string, value string) error {
	ModelRequestRateLimitMutex.Lock()
	defer ModelRequestRateLimitMutex.Unlock()
	return applyDistillationRateLimitOption(&modelRequestRateLimitDistillationSettings, key, value)
}

func parseModelRequestRateLimitGroup(jsonStr string) (map[string][2]int, error) {
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = "{}"
	}
	groupLimits := make(map[string][2]int)
	if err := common.UnmarshalJsonStr(jsonStr, &groupLimits); err != nil {
		return nil, err
	}
	if groupLimits == nil {
		return nil, fmt.Errorf("group rate limits must be a JSON object")
	}
	for group, limits := range groupLimits {
		if err := validateRateLimitPair("group "+group, limits); err != nil {
			return nil, err
		}
	}
	return groupLimits, nil
}

func parseModelRequestRateLimitUser(jsonStr string) (map[int][2]int, error) {
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = "{}"
	}
	serializedLimits := make(map[string][2]int)
	if err := common.UnmarshalJsonStr(jsonStr, &serializedLimits); err != nil {
		return nil, err
	}
	if serializedLimits == nil {
		return nil, fmt.Errorf("user rate limits must be a JSON object")
	}

	userLimits := make(map[int][2]int, len(serializedLimits))
	for userIDText, limits := range serializedLimits {
		userID, err := strconv.Atoi(userIDText)
		if err != nil || userID <= 0 || strconv.Itoa(userID) != userIDText {
			return nil, fmt.Errorf("user rate limit key %q must be a positive decimal user ID", userIDText)
		}
		if err := validateRateLimitPair("user "+userIDText, limits); err != nil {
			return nil, err
		}
		userLimits[userID] = limits
	}
	return userLimits, nil
}

func validateRateLimitPair(label string, limits [2]int) error {
	if limits[0] < 0 || limits[1] < 1 {
		return fmt.Errorf("%s has invalid rate limit values: [%d, %d]", label, limits[0], limits[1])
	}
	if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
		return fmt.Errorf("%s [%d, %d] exceeds the maximum rate limit value %d", label, limits[0], limits[1], math.MaxInt32)
	}
	return nil
}

func validateRateLimitInteger(value string, minimum int, label string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%s must be an integer", label)
	}
	if parsed < minimum || parsed > math.MaxInt32 {
		return fmt.Errorf("%s must be between %d and %d", label, minimum, math.MaxInt32)
	}
	return nil
}

func applyDistillationRateLimitOption(
	settings *DistillationRateLimitSettings,
	key string,
	value string,
) error {
	if key == "ModelRequestRateLimitDistillationEnabled" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("distillation enabled must be a boolean")
		}
		settings.Enabled = enabled
		return nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("distillation setting must be an integer")
	}
	switch key {
	case "ModelRequestRateLimitDistillationThreshold":
		settings.Threshold = parsed
	case "ModelRequestRateLimitDistillationRPM":
		settings.RPM = parsed
	case "ModelRequestRateLimitDistillationPenaltyMinutes":
		settings.PenaltyMinutes = parsed
	case "ModelRequestRateLimitDistillationObservationMinutes":
		settings.ObservationMinutes = parsed
	default:
		return fmt.Errorf("unsupported distillation rate limit option %q", key)
	}
	return nil
}
