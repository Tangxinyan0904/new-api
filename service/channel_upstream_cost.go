package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	upstreamCostRequestTimeout = 20 * time.Second
	upstreamCostMaxBodyBytes   = 4 << 20
	upstreamCostCacheTTL       = 5 * time.Minute
	upstreamCostErrorCacheTTL  = time.Minute
	// upstreamCostConcurrency bounds how many channels are queried at once.
	upstreamCostConcurrency = 6
	// newAPIDefaultQuotaPerUnit is new-api's quota per dollar unless the
	// upstream reports another one.
	newAPIDefaultQuotaPerUnit = 500000
	// newAPILookupCacheTTL keeps token names and quota units, which rarely
	// change. new-api allows only 20 token lookups per 20 minutes per IP.
	newAPILookupCacheTTL = 6 * time.Hour
)

var (
	ErrUpstreamNoKey          = common.NewMessage("The channel has no key")
	ErrUpstreamNoBaseURL      = common.NewMessage("The channel has no address")
	ErrUpstreamNoDailyUsage   = common.NewMessage("This sub2api does not report daily usage. Update it to a newer version.")
	ErrUpstreamBaseURLChanged = common.NewMessage("The channel address changed. Enter the access token again.")
	ErrUpstreamTokenUnknown   = common.NewMessage("The upstream did not return the key's name")
)

// UpstreamCostRanges are the ranges of the cost statistics page, in the
// viewer's time zone. The week starts six days before today.
type UpstreamCostRanges struct {
	TodayStart     int64
	YesterdayStart int64
	WeekStart      int64
	Now            int64
	// Timezone is the viewer's IANA time zone; sub2api splits days with it.
	Timezone string
}

// UpstreamCost is what an upstream charged, in the upstream's dollars.
type UpstreamCost struct {
	Today     float64 `json:"today"`
	Yesterday float64 `json:"yesterday"`
	Week      float64 `json:"week"`
	// Balance is what is left on the upstream account, nil when the upstream
	// does not say.
	Balance *float64 `json:"balance"`
}

// UpstreamCostResult is a channel's upstream cost or why it could not be read.
type UpstreamCostResult struct {
	Cost      UpstreamCost
	Err       error
	FetchedAt int64
}

type upstreamCostCacheEntry struct {
	result  UpstreamCostResult
	expires time.Time
}

var (
	upstreamCostCacheMu sync.Mutex
	upstreamCostCache   = map[string]upstreamCostCacheEntry{}

	newAPILookupCacheMu sync.Mutex
	newAPILookupCache   = map[string]newAPILookupCacheEntry{}
)

type newAPILookupCacheEntry struct {
	value   string
	expires time.Time
}

// cachedNewAPILookup returns lookup's answer for key, asking again only after
// newAPILookupCacheTTL. Failures are not kept.
func cachedNewAPILookup(key string, lookup func() (string, error)) (string, error) {
	newAPILookupCacheMu.Lock()
	entry, cached := newAPILookupCache[key]
	newAPILookupCacheMu.Unlock()
	if cached && time.Now().Before(entry.expires) {
		return entry.value, nil
	}
	value, err := lookup()
	if err != nil {
		return "", err
	}
	newAPILookupCacheMu.Lock()
	newAPILookupCache[key] = newAPILookupCacheEntry{value: value, expires: time.Now().Add(newAPILookupCacheTTL)}
	newAPILookupCacheMu.Unlock()
	return value, nil
}

// upstreamGet sends a GET to the channel's address through the channel's
// proxy and decodes the JSON answer into out.
func upstreamGet(ctx context.Context, channel *model.Channel, path string, headers map[string]string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, upstreamCostRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(channel.GetBaseURL(), "/")+path, nil)
	if err != nil {
		return common.NewMessage("Upstream unreachable: {{error}}", map[string]any{"error": err.Error()})
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	setting := channel.GetSetting()
	client, err := GetHttpClientWithProxySettings(setting.Proxy, setting)
	if err != nil {
		return common.NewMessage("Upstream unreachable: {{error}}", map[string]any{"error": err.Error()})
	}
	response, err := client.Do(request)
	if err != nil {
		return common.NewMessage("Upstream unreachable: {{error}}", map[string]any{"error": err.Error()})
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, upstreamCostMaxBodyBytes))
	if err != nil {
		return common.NewMessage("Upstream unreachable: {{error}}", map[string]any{"error": err.Error()})
	}
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Message string `json:"message"`
			Error   struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = common.Unmarshal(body, &failure)
		message := cmp.Or(failure.Error.Message, failure.Message, http.StatusText(response.StatusCode))
		return common.NewMessage("Upstream answered {{status}}: {{error}}", map[string]any{"status": response.StatusCode, "error": truncateUpstreamMessage(message)})
	}
	if err := common.Unmarshal(body, out); err != nil {
		return common.NewMessage("The upstream answer could not be read. Check the upstream type.")
	}
	return nil
}

func truncateUpstreamMessage(message string) string {
	runes := []rune(message)
	if len(runes) > 200 {
		return string(runes[:200]) + "…"
	}
	return message
}

// channelUpstreamKeys returns the channel's distinct keys.
func channelUpstreamKeys(channel *model.Channel) []string {
	keys := make([]string, 0)
	for _, key := range channel.GetKeys() {
		key = strings.TrimSpace(key)
		if key != "" && !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

// sub2apiWindowCost returns what a sub2api key was charged from the start of
// the day days-1 days ago, in timezone, until now, and what the key can still
// spend: the wallet balance, the subscription's remainder or the key's own
// limit, whichever applies to it.
func sub2apiWindowCost(ctx context.Context, channel *model.Channel, key string, days int, timezone string) (float64, *float64, error) {
	var response struct {
		Remaining  *float64 `json:"remaining"`
		DailyUsage *[]struct {
			ActualCost float64 `json:"actual_cost"`
		} `json:"daily_usage"`
	}
	path := "/v1/usage?days=" + strconv.Itoa(days) + "&timezone=" + url.QueryEscape(timezone)
	if err := upstreamGet(ctx, channel, path, map[string]string{"Authorization": "Bearer " + key}, &response); err != nil {
		return 0, nil, err
	}
	if response.DailyUsage == nil {
		return 0, nil, ErrUpstreamNoDailyUsage
	}
	total := 0.0
	for _, day := range *response.DailyUsage {
		total += day.ActualCost
	}
	return total, response.Remaining, nil
}

// sub2apiCost reads the spend of the channel's keys. sub2api may group its
// daily rows by its own time zone, so each range is taken from windows that
// start at the viewer's midnight: yesterday is the two-day window minus today.
func sub2apiCost(ctx context.Context, channel *model.Channel, ranges UpstreamCostRanges) (UpstreamCost, error) {
	keys := channelUpstreamKeys(channel)
	if len(keys) == 0 {
		return UpstreamCost{}, ErrUpstreamNoKey
	}
	cost := UpstreamCost{}
	for _, key := range keys {
		today, remaining, err := sub2apiWindowCost(ctx, channel, key, 1, ranges.Timezone)
		if err != nil {
			return UpstreamCost{}, err
		}
		twoDays, _, err := sub2apiWindowCost(ctx, channel, key, 2, ranges.Timezone)
		if err != nil {
			return UpstreamCost{}, err
		}
		week, _, err := sub2apiWindowCost(ctx, channel, key, 7, ranges.Timezone)
		if err != nil {
			return UpstreamCost{}, err
		}
		cost.Today += today
		cost.Yesterday += max(twoDays-today, 0)
		cost.Week += week
		// Keys of one account share its balance; keep the first one reported.
		if cost.Balance == nil {
			cost.Balance = remaining
		}
	}
	return cost, nil
}

// newAPITokenName looks up the name of a new-api key; account statistics are
// filtered by token name.
func newAPITokenName(ctx context.Context, channel *model.Channel, key string) (string, error) {
	return cachedNewAPILookup("name|"+channel.GetBaseURL()+"|"+key, func() (string, error) {
		var response struct {
			Data struct {
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := upstreamGet(ctx, channel, "/api/usage/token/", map[string]string{"Authorization": "Bearer " + key}, &response); err != nil {
			return "", err
		}
		if response.Data.Name == "" {
			return "", ErrUpstreamTokenUnknown
		}
		return response.Data.Name, nil
	})
}

func newAPIQuotaPerUnit(ctx context.Context, channel *model.Channel) (float64, error) {
	value, err := cachedNewAPILookup("unit|"+channel.GetBaseURL(), func() (string, error) {
		var response struct {
			Data struct {
				QuotaPerUnit float64 `json:"quota_per_unit"`
			} `json:"data"`
		}
		if err := upstreamGet(ctx, channel, "/api/status", nil, &response); err != nil {
			return "", err
		}
		if response.Data.QuotaPerUnit <= 0 {
			return strconv.Itoa(newAPIDefaultQuotaPerUnit), nil
		}
		return strconv.FormatFloat(response.Data.QuotaPerUnit, 'f', -1, 64), nil
	})
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(value, 64)
}

// newAPIBalance returns the quota left on the new-api account the access token
// belongs to, in the upstream's quota units.
func newAPIBalance(ctx context.Context, channel *model.Channel, billing model.ChannelUpstreamBilling) (float64, error) {
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Quota float64 `json:"quota"`
		} `json:"data"`
	}
	headers := map[string]string{
		"Authorization": "Bearer " + billing.AccessToken,
		"New-Api-User":  strconv.Itoa(billing.UpstreamUserId),
	}
	if err := upstreamGet(ctx, channel, "/api/user/self", headers, &response); err != nil {
		return 0, err
	}
	if !response.Success {
		return 0, common.NewMessage("Upstream answered {{status}}: {{error}}", map[string]any{"status": http.StatusOK, "error": truncateUpstreamMessage(response.Message)})
	}
	return response.Data.Quota, nil
}

// newAPIUsedQuota sums what the account's tokens named tokenName consumed from
// start to end inclusive, in the upstream's quota units.
func newAPIUsedQuota(ctx context.Context, channel *model.Channel, billing model.ChannelUpstreamBilling, tokenName string, start, end int64) (float64, error) {
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Quota float64 `json:"quota"`
		} `json:"data"`
	}
	path := fmt.Sprintf("/api/log/self/stat?type=%d&start_timestamp=%d&end_timestamp=%d&token_name=%s",
		model.LogTypeConsume, start, end, url.QueryEscape(tokenName))
	headers := map[string]string{
		"Authorization": "Bearer " + billing.AccessToken,
		"New-Api-User":  strconv.Itoa(billing.UpstreamUserId),
	}
	if err := upstreamGet(ctx, channel, path, headers, &response); err != nil {
		return 0, err
	}
	if !response.Success {
		return 0, common.NewMessage("Upstream answered {{status}}: {{error}}", map[string]any{"status": http.StatusOK, "error": truncateUpstreamMessage(response.Message)})
	}
	return response.Data.Quota, nil
}

// newAPICost reads the spend of the channel's keys from the new-api account
// the access token belongs to.
func newAPICost(ctx context.Context, channel *model.Channel, billing model.ChannelUpstreamBilling, ranges UpstreamCostRanges) (UpstreamCost, error) {
	if billing.BaseURL != channel.GetBaseURL() {
		return UpstreamCost{}, ErrUpstreamBaseURLChanged
	}
	keys := channelUpstreamKeys(channel)
	if len(keys) == 0 {
		return UpstreamCost{}, ErrUpstreamNoKey
	}
	quotaPerUnit, err := newAPIQuotaPerUnit(ctx, channel)
	if err != nil {
		return UpstreamCost{}, err
	}
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		name, err := newAPITokenName(ctx, channel, key)
		if err != nil {
			return UpstreamCost{}, err
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	cost := UpstreamCost{}
	for _, name := range names {
		today, err := newAPIUsedQuota(ctx, channel, billing, name, ranges.TodayStart, ranges.Now)
		if err != nil {
			return UpstreamCost{}, err
		}
		yesterday, err := newAPIUsedQuota(ctx, channel, billing, name, ranges.YesterdayStart, ranges.TodayStart-1)
		if err != nil {
			return UpstreamCost{}, err
		}
		week, err := newAPIUsedQuota(ctx, channel, billing, name, ranges.WeekStart, ranges.Now)
		if err != nil {
			return UpstreamCost{}, err
		}
		cost.Today += today / quotaPerUnit
		cost.Yesterday += yesterday / quotaPerUnit
		cost.Week += week / quotaPerUnit
	}
	balance, err := newAPIBalance(ctx, channel, billing)
	if err != nil {
		return UpstreamCost{}, err
	}
	balance /= quotaPerUnit
	cost.Balance = &balance
	return cost, nil
}

// QueryChannelUpstreamCost reads what the channel's upstream charged in the
// ranges.
func QueryChannelUpstreamCost(ctx context.Context, channel *model.Channel, billing model.ChannelUpstreamBilling, ranges UpstreamCostRanges) (UpstreamCost, error) {
	if channel.GetBaseURL() == "" {
		return UpstreamCost{}, ErrUpstreamNoBaseURL
	}
	switch billing.Kind {
	case model.UpstreamBillingSub2API:
		return sub2apiCost(ctx, channel, ranges)
	case model.UpstreamBillingNewAPI:
		return newAPICost(ctx, channel, billing, ranges)
	}
	return UpstreamCost{}, errors.New("unknown upstream billing kind")
}

// CheckChannelUpstreamBilling makes a cheap request with billing so a wrong
// upstream type, key or access token is reported when it is saved.
func CheckChannelUpstreamBilling(ctx context.Context, channel *model.Channel, billing model.ChannelUpstreamBilling) error {
	if channel.GetBaseURL() == "" {
		return ErrUpstreamNoBaseURL
	}
	keys := channelUpstreamKeys(channel)
	if len(keys) == 0 {
		return ErrUpstreamNoKey
	}
	if billing.Kind == model.UpstreamBillingSub2API {
		_, _, err := sub2apiWindowCost(ctx, channel, keys[0], 1, "UTC")
		return err
	}
	name, err := newAPITokenName(ctx, channel, keys[0])
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	_, err = newAPIUsedQuota(ctx, channel, billing, name, now-60, now)
	return err
}

// QueryChannelUpstreamCosts reads the upstream cost of each channel that has
// upstream billing, a few channels at a time. Results are cached for five
// minutes, failures for one, unless refresh is set.
func QueryChannelUpstreamCosts(ctx context.Context, channels []*model.Channel, billings map[int]model.ChannelUpstreamBilling, ranges UpstreamCostRanges, refresh bool) map[int]UpstreamCostResult {
	results := make(map[int]UpstreamCostResult, len(channels))
	var resultsMu sync.Mutex
	slots := make(chan struct{}, upstreamCostConcurrency)
	var wg sync.WaitGroup
	for _, channel := range channels {
		billing, ok := billings[channel.Id]
		if !ok {
			continue
		}
		cacheKey := fmt.Sprintf("%d|%d|%d|%s|%d", channel.Id, ranges.TodayStart, ranges.WeekStart, ranges.Timezone, billing.UpdatedAt)
		if !refresh {
			upstreamCostCacheMu.Lock()
			entry, cached := upstreamCostCache[cacheKey]
			upstreamCostCacheMu.Unlock()
			if cached && time.Now().Before(entry.expires) {
				results[channel.Id] = entry.result
				continue
			}
		}
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			cost, err := QueryChannelUpstreamCost(ctx, channel, billing, ranges)
			result := UpstreamCostResult{Cost: cost, Err: err, FetchedAt: common.GetTimestamp()}
			ttl := upstreamCostCacheTTL
			if err != nil {
				ttl = upstreamCostErrorCacheTTL
			}
			upstreamCostCacheMu.Lock()
			upstreamCostCache[cacheKey] = upstreamCostCacheEntry{result: result, expires: time.Now().Add(ttl)}
			for key, entry := range upstreamCostCache {
				if time.Now().After(entry.expires) {
					delete(upstreamCostCache, key)
				}
			}
			upstreamCostCacheMu.Unlock()
			resultsMu.Lock()
			results[channel.Id] = result
			resultsMu.Unlock()
		})
	}
	wg.Wait()
	return results
}

// UpstreamAccountForecast says how long an upstream account's balance lasts
// at its recent spending. Channels on the same upstream address share one
// sub2api account; new-api accounts are also told apart by user id.
type UpstreamAccountForecast struct {
	Kind       string `json:"kind"`
	BaseURL    string `json:"base_url"`
	ChannelIds []int  `json:"channel_ids"`
	// Balance and DailySpend are in the upstream's dollars.
	Balance    float64 `json:"balance"`
	DailySpend float64 `json:"daily_spend"`
	// DaysLeft is nil when nothing was spent recently.
	DaysLeft *float64 `json:"days_left"`
	// Low is set when the balance is used up or lasts less than a day.
	Low bool `json:"low"`
}

// ForecastUpstreamAccounts groups the channels that were read successfully by
// upstream account. The daily spend averages yesterday and today so far.
func ForecastUpstreamAccounts(channels []*model.Channel, billings map[int]model.ChannelUpstreamBilling, results map[int]UpstreamCostResult, ranges UpstreamCostRanges) []UpstreamAccountForecast {
	forecasts := make([]UpstreamAccountForecast, 0)
	index := make(map[string]int)
	spent := make(map[string]float64)
	for _, channel := range channels {
		result, ok := results[channel.Id]
		billing := billings[channel.Id]
		if !ok || result.Err != nil || result.Cost.Balance == nil {
			continue
		}
		key := billing.Kind + "|" + channel.GetBaseURL()
		if billing.Kind == model.UpstreamBillingNewAPI {
			key += "|" + strconv.Itoa(billing.UpstreamUserId)
		}
		position, seen := index[key]
		if !seen {
			position = len(forecasts)
			index[key] = position
			forecasts = append(forecasts, UpstreamAccountForecast{
				Kind:       billing.Kind,
				BaseURL:    channel.GetBaseURL(),
				ChannelIds: []int{},
				Balance:    *result.Cost.Balance,
			})
		}
		forecasts[position].ChannelIds = append(forecasts[position].ChannelIds, channel.Id)
		spent[key] += result.Cost.Today + result.Cost.Yesterday
	}
	elapsedDays := float64(ranges.Now-ranges.YesterdayStart) / 86400
	for key, position := range index {
		forecast := &forecasts[position]
		slices.Sort(forecast.ChannelIds)
		if elapsedDays > 0 {
			forecast.DailySpend = spent[key] / elapsedDays
		}
		if forecast.DailySpend > 0 {
			daysLeft := max(forecast.Balance, 0) / forecast.DailySpend
			forecast.DaysLeft = &daysLeft
		}
		forecast.Low = forecast.Balance <= 0 || (forecast.DaysLeft != nil && *forecast.DaysLeft < 1)
	}
	slices.SortFunc(forecasts, func(a, b UpstreamAccountForecast) int {
		return cmp.Compare(a.ChannelIds[0], b.ChannelIds[0])
	})
	return forecasts
}
