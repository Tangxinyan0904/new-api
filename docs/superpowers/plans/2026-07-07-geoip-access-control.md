# GeoIP Access Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub-compatible GeoIP country blocking to new-api with backend enforcement, database download support, a default frontend settings section, and homepage popup behavior.

**Architecture:** Store GeoIP settings in the existing option system, centralize runtime decisions in a `service/geoip` package, and enforce route behavior with one global middleware that classifies requests by path. The default frontend reads the same options for admin configuration and calls a public GeoIP status endpoint for homepage popups.

**Tech Stack:** Go, Gin, GORM option storage, `github.com/oschwald/maxminddb-golang`, React, TanStack Query, react-hook-form, zod, Base UI/Shadcn-style components.

---

## File Structure

- Create `setting/geoip_setting/setting.go`: typed GeoIP options, defaults, mode/country validation, JSON serialization.
- Create `setting/geoip_setting/setting_test.go`: unit tests for mode and country normalization.
- Create `service/geoip.go`: MaxMind DB reader cache, request decision logic, downloader and archive extraction helpers.
- Create `service/geoip_test.go`: unit tests for decision logic and archive extraction.
- Create `middleware/geoip.go`: global request classifier and rejection middleware.
- Create `middleware/geoip_test.go`: middleware tests for API/web behavior.
- Create `controller/geoip.go`: public status endpoint and root-admin download endpoint.
- Modify `main.go`: register GeoIP middleware globally after request context setup.
- Modify `router/api-router.go`: add `/api/geoip/status` and `/api/option/geoip/download`.
- Modify `model/option.go`: add default option keys and runtime update hooks.
- Modify `controller/option.go`: validate GeoIP option updates.
- Modify `go.mod`/`go.sum`: add MaxMind DB dependency.
- Create `web/default/src/features/system-settings/request-limits/geoip-section.tsx`: admin UI matching the screenshot.
- Create `web/default/src/features/system-settings/request-limits/countries.ts`: country option list with `CN`.
- Modify `web/default/src/features/system-settings/types.ts`: GeoIP setting and response types.
- Modify `web/default/src/features/system-settings/api.ts`: download API.
- Modify `web/default/src/features/system-settings/security/index.tsx`: default GeoIP settings.
- Modify `web/default/src/features/system-settings/security/section-registry.tsx`: add GeoIP security section.
- Modify `web/default/src/features/home/api.ts` and `web/default/src/features/home/types.ts`: GeoIP status API types.
- Modify `web/default/src/features/home/index.tsx`: display dismissible/non-dismissible GeoIP popup.

---

### Task 1: GeoIP Settings Model

**Files:**
- Create: `setting/geoip_setting/setting.go`
- Create: `setting/geoip_setting/setting_test.go`
- Modify: `model/option.go`
- Modify: `controller/option.go`

- [ ] **Step 1: Write failing settings tests**

Create tests that define expected behavior:

```go
func TestNormalizeBlockedCountriesJSON(t *testing.T) {
	got, err := geoip_setting.NormalizeBlockedCountriesJSON(`["cn"," US ","cn"]`)
	require.NoError(t, err)
	require.JSONEq(t, `["CN","US"]`, got)
}

func TestNormalizeBlockedCountriesJSONRejectsInvalidCode(t *testing.T) {
	_, err := geoip_setting.NormalizeBlockedCountriesJSON(`["china"]`)
	require.Error(t, err)
}

func TestValidateModeRejectsUnknownMode(t *testing.T) {
	require.NoError(t, geoip_setting.ValidateMode(geoip_setting.ModeOff))
	require.Error(t, geoip_setting.ValidateMode("unknown"))
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./setting/geoip_setting`

Expected: fails because the package does not exist.

- [ ] **Step 3: Implement settings package**

Add constants and helpers:

```go
const (
	ModeOff                    = "off"
	ModeHomepageNotice         = "homepage_notice"
	ModeHomepageBlock          = "homepage_block"
	ModeHomepageBlockAPIReject = "homepage_block_api_reject"
	ModeFullReject             = "full_reject"
)
```

Expose mutable runtime defaults:

```go
var Mode = ModeOff
var DatabasePath = "Country.mmdb"
var DownloadURL = ""
var MaxMindLicenseKey = ""
var PopupMessage = "Your current region is not supported by this service. Please contact the administrator if you believe this is a mistake."
var AllowPrivateLoopback = true
var BlockedCountries = []string{"CN"}
```

Implement `ValidateMode`, `NormalizeBlockedCountriesJSON`, `UpdateBlockedCountriesByJSONString`, and `BlockedCountries2JSONString`.

- [ ] **Step 4: Wire option defaults**

In `model.InitOptionMap`, add:

```go
common.OptionMap["geoip.mode"] = geoip_setting.Mode
common.OptionMap["geoip.database_path"] = geoip_setting.DatabasePath
common.OptionMap["geoip.download_url"] = geoip_setting.DownloadURL
common.OptionMap["geoip.maxmind_license_key"] = geoip_setting.MaxMindLicenseKey
common.OptionMap["geoip.popup_message"] = geoip_setting.PopupMessage
common.OptionMap["geoip.allow_private_loopback"] = strconv.FormatBool(geoip_setting.AllowPrivateLoopback)
common.OptionMap["geoip.blocked_countries"] = geoip_setting.BlockedCountries2JSONString()
```

In `updateOptionMap`, update GeoIP runtime values for those keys.

- [ ] **Step 5: Validate option saves**

In `controller.UpdateOption`, reject invalid modes and invalid country JSON before `model.UpdateOption`.

- [ ] **Step 6: Run settings tests**

Run: `go test ./setting/geoip_setting ./model`

Expected: pass.

---

### Task 2: GeoIP Service and Downloader

**Files:**
- Create: `service/geoip.go`
- Create: `service/geoip_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add MaxMind DB dependency**

Run: `go get github.com/oschwald/maxminddb-golang`

- [ ] **Step 2: Write failing decision tests**

Create tests with an injectable lookup function:

```go
func TestGeoIPDecisionBlocksConfiguredCountry(t *testing.T) {
	decision := service.DecideGeoIPAccessForCountry(service.GeoIPDecisionInput{
		Mode:             geoip_setting.ModeHomepageBlockAPIReject,
		CountryCode:      "CN",
		BlockedCountries: []string{"CN"},
		DatabaseReady:    true,
		Message:          "blocked",
	})
	require.True(t, decision.Blocked)
	require.True(t, decision.RejectsAPI())
}

func TestGeoIPDecisionFailsOpenWhenDatabaseMissing(t *testing.T) {
	decision := service.DecideGeoIPAccessForCountry(service.GeoIPDecisionInput{
		Mode:             geoip_setting.ModeFullReject,
		CountryCode:      "",
		BlockedCountries: []string{"CN"},
		DatabaseReady:    false,
		Message:          "blocked",
	})
	require.False(t, decision.Blocked)
	require.False(t, decision.DatabaseReady)
}
```

- [ ] **Step 3: Run red test**

Run: `go test ./service -run GeoIP`

Expected: fails because service functions are missing.

- [ ] **Step 4: Implement decision model**

Create:

```go
type GeoIPDecision struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`
	Blocked       bool   `json:"blocked"`
	CountryCode   string `json:"country_code"`
	Message       string `json:"message"`
	DatabaseReady bool   `json:"database_ready"`
}
```

Add methods `RejectsAPI()` and `RejectsWeb()` based on mode.

- [ ] **Step 5: Implement reader cache**

Use `maxminddb.Open(path)` with a mutex and reload when the configured path or file modified time changes. Lookup `country.iso_code`, falling back to `registered_country.iso_code`.

- [ ] **Step 6: Implement private/loopback bypass**

Use `net.ParseIP`, `ip.IsLoopback()`, `ip.IsPrivate()`, `ip.IsUnspecified()`, and existing private IP helpers where useful. When bypass applies, return enabled decision with `Blocked=false`.

- [ ] **Step 7: Write failing downloader tests**

Create temporary `.mmdb`, `.mmdb.gz`, `.tar.gz`, and `.zip` fixtures using simple bytes and test that extraction selects a `.mmdb` entry. Validation by `maxminddb.Open` should be isolated behind a validator function so archive extraction can be tested without a real database.

- [ ] **Step 8: Implement downloader**

Download via the existing protected fetch client, support GitHub/MaxMind direct URLs, extract to a temp file, validate `.mmdb`, and replace the configured database path.

- [ ] **Step 9: Run service tests**

Run: `go test ./service -run GeoIP`

Expected: pass.

---

### Task 3: Controller and Middleware

**Files:**
- Create: `controller/geoip.go`
- Create: `middleware/geoip.go`
- Create: `middleware/geoip_test.go`
- Modify: `router/api-router.go`
- Modify: `main.go`

- [ ] **Step 1: Write failing middleware tests**

Test behavior:

```go
func TestGeoIPMiddlewareRejectsBlockedAPIMode(t *testing.T) {
	router := gin.New()
	router.Use(middleware.GeoIPAccessWithResolver(func(*gin.Context) service.GeoIPDecision {
		return service.GeoIPDecision{
			Enabled: true, Mode: geoip_setting.ModeHomepageBlockAPIReject,
			Blocked: true, Message: "blocked", DatabaseReady: true,
		}
	}))
	router.GET("/api/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
```

Also test that `/api/geoip/status` is exempt and that web paths are allowed in `homepage_block_api_reject`.

- [ ] **Step 2: Run red test**

Run: `go test ./middleware -run GeoIP`

Expected: fails because middleware is missing.

- [ ] **Step 3: Implement middleware**

Classify request paths:

- `/api/geoip/status`: exempt.
- `/api/*`: API.
- `/v1/*`, `/v1beta/*`, `/mj/*`, `/suno/*`, `/kling/*`, `/jimeng/*`: relay/API traffic.
- everything else: web.

Reject with JSON for API/relay and plain text for web.

- [ ] **Step 4: Implement controllers**

`GetGeoIPStatus` returns `service.ResolveGeoIPDecision(c.ClientIP())`.

`DownloadGeoIPDatabase` calls `service.DownloadGeoIPDatabase(c.Request.Context())`.

- [ ] **Step 5: Register routes**

In `router/api-router.go`:

```go
apiRouter.GET("/geoip/status", controller.GetGeoIPStatus)
optionRoute.POST("/geoip/download", controller.DownloadGeoIPDatabase)
```

In `main.go`, register global middleware after `middleware.I18n()` and before logger/session setup:

```go
server.Use(middleware.GeoIPAccess())
```

- [ ] **Step 6: Run controller/middleware tests**

Run: `go test ./middleware ./controller`

Expected: pass.

---

### Task 4: Default Frontend Settings Section

**Files:**
- Create: `web/default/src/features/system-settings/request-limits/geoip-section.tsx`
- Create: `web/default/src/features/system-settings/request-limits/countries.ts`
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/api.ts`
- Modify: `web/default/src/features/system-settings/security/index.tsx`
- Modify: `web/default/src/features/system-settings/security/section-registry.tsx`

- [ ] **Step 1: Add types and API helpers**

Add settings keys to `SecuritySettings`. Add:

```ts
export async function downloadGeoIPDatabase() {
  const res = await api.post<UpdateOptionResponse>('/api/option/geoip/download')
  return res.data
}
```

- [ ] **Step 2: Add country options**

Create `countries.ts` with at least `CN`, `US`, `JP`, `KR`, `SG`, `HK`, `TW`, `RU`, `GB`, `DE`, `FR`, `CA`, `AU`. Labels should use the format `中国 / China (CN)` for CN.

- [ ] **Step 3: Create form UI**

Use existing `SettingsForm`, `SettingsFormGrid`, `SettingsControlGroup`, `MultiSelect`, `Button`, `Input`, `Textarea`, and `RadioGroup`.

Form fields:

- `geoip.mode`
- `geoip.database_path`
- `geoip.download_url`
- `geoip.maxmind_license_key`
- `geoip.popup_message`
- `geoip.allow_private_loopback`
- `geoip.blocked_countries`

Save changed fields with `useUpdateOption`, serializing countries as JSON.

- [ ] **Step 4: Register section**

Add `{ id: 'geoip', titleKey: 'GeoIP Access Restriction' }` to the security registry and pass GeoIP default values.

- [ ] **Step 5: Typecheck frontend**

Run: `C:\Users\Administrator\.cherrystudio\bin\bun.exe run typecheck` in `web/default`.

Expected: pass.

---

### Task 5: Homepage Popup

**Files:**
- Modify: `web/default/src/features/home/api.ts`
- Modify: `web/default/src/features/home/types.ts`
- Modify: `web/default/src/features/home/index.tsx`

- [ ] **Step 1: Add API type and function**

Add:

```ts
export type GeoIPStatus = {
  enabled: boolean
  mode: string
  blocked: boolean
  country_code: string
  message: string
  database_ready: boolean
}
```

Add `getGeoIPStatus()`.

- [ ] **Step 2: Add popup state**

Fetch status on home mount. Show dialog only when `enabled && blocked && mode !== 'full_reject'`.

- [ ] **Step 3: Implement dismiss behavior**

Dismissible only for `homepage_notice`. Non-dismissible for `homepage_block` and `homepage_block_api_reject`.

- [ ] **Step 4: Typecheck frontend**

Run: `C:\Users\Administrator\.cherrystudio\bin\bun.exe run typecheck` in `web/default`.

Expected: pass.

---

### Task 6: Verification and Commit

**Files:**
- All modified files.

- [ ] **Step 1: Run backend tests**

Run: `go test ./...`

Expected: pass.

- [ ] **Step 2: Run default frontend checks**

Run in `web/default`:

```powershell
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run typecheck
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run build
```

Expected: both pass.

- [ ] **Step 3: Check git status**

Run:

```powershell
git status --porcelain=v1 --untracked-files=all
```

Expected: only intended source/doc files are staged or modified; local `tmp/`, `output/`, database and logs remain untracked.

- [ ] **Step 4: Commit**

Run:

```powershell
git add setting/geoip_setting service middleware controller router main.go model/option.go controller/option.go go.mod go.sum web/default/src/features/system-settings web/default/src/features/home docs/superpowers/plans/2026-07-07-geoip-access-control.md
git commit -m "feat: add geoip access control"
```
