# GeoIP Access Control Design

## Goal

Integrate GeoIP access control into new-api so administrators can configure country/region blocking from the default frontend, download a GeoIP country database from GitHub-hosted projects or MaxMind-compatible URLs, and apply the chosen restriction mode consistently to homepage and API traffic.

## Defaults

- GeoIP access mode: `off`.
- GeoIP database path: `Country.mmdb`.
- Blocked countries/regions: `["CN"]`.
- Popup message: `Your current region is not supported by this service. Please contact the administrator if you believe this is a mistake.`
- Private and loopback IP bypass: enabled by default.

These defaults preserve current behavior after upgrade because the feature is disabled until an administrator enables a mode.

## Access Modes

Use a string option key `geoip.mode` with these values:

- `off`: do not apply GeoIP checks.
- `homepage_notice`: homepage shows a dismissible popup for blocked countries. API requests are allowed.
- `homepage_block`: homepage shows a non-dismissible popup for blocked countries. API requests are allowed.
- `homepage_block_api_reject`: homepage shows a non-dismissible popup and blocked country API/relay requests return `403`.
- `full_reject`: blocked country web, API, and relay requests return a server-side rejection.

The public GeoIP status endpoint is exempt from GeoIP rejection so the frontend can render the configured popup state.

## Data Source Support

The downloader accepts a direct URL from `geoip.download_url` and supports:

- `.mmdb`
- `.mmdb.gz`
- `.tar.gz` or `.tgz` containing a `.mmdb`
- `.zip` containing a `.mmdb`

The URL may point to GitHub Release assets, GitHub raw files, jsDelivr mirrors, or MaxMind direct download URLs. MaxMind license support remains available through `geoip.maxmind_license_key`, but the implementation must not require MaxMind when a GitHub project URL is configured.

Known compatible GitHub-hosted sources include:

- P3TERX/GeoLite.mmdb release assets such as `GeoLite2-Country.mmdb`.
- Loyalsoldier/geoip raw or jsDelivr assets such as `Country.mmdb`.
- wp-statistics/GeoLite2-Country npm/jsDelivr assets such as `GeoLite2-Country.mmdb.gz`.

The downloader writes to a temporary file in the target directory and atomically replaces the configured database path only after extraction and validation succeed.

## Backend Settings

Add a focused GeoIP settings package, for example `setting/geoip_setting`, with these persisted options:

- `geoip.mode`
- `geoip.database_path`
- `geoip.download_url`
- `geoip.maxmind_license_key`
- `geoip.popup_message`
- `geoip.allow_private_loopback`
- `geoip.blocked_countries`

`geoip.maxmind_license_key` is saved through the existing option API but not returned from `GET /api/option/` because existing option filtering hides keys ending with `Key`.

`geoip.blocked_countries` is a JSON string array of ISO 3166-1 alpha-2 country codes. Values are normalized to uppercase and invalid values are rejected on save.

## GeoIP Service

Add a service layer responsible for:

- Loading and caching the MaxMind DB reader.
- Reloading when the configured path changes or a new database is downloaded.
- Resolving request IP country code.
- Skipping private and loopback IPs when `geoip.allow_private_loopback` is enabled.
- Returning a structured decision containing `blocked`, `country_code`, `mode`, and `message`.

If the mode is not `off` and the database is missing or unreadable, fail open for traffic and report `database_ready: false` in the status endpoint. This avoids accidentally locking out operators due to a missing local database.

## Request Enforcement

Add GeoIP middleware with route-aware behavior:

- API and relay routes reject only when mode is `homepage_block_api_reject` or `full_reject`.
- Web/static requests reject only when mode is `full_reject`.
- Homepage popup behavior is handled in the frontend using the public status endpoint.
- Local/private IP bypass applies before database lookup.
- Rejection responses use HTTP `403` and the configured popup message.

Register middleware early enough to cover `/api`, relay routes, and static web responses, but after request ID and language setup so responses and logs have normal context.

## API Endpoints

Add public endpoint:

- `GET /api/geoip/status`

Response fields:

- `enabled`
- `mode`
- `blocked`
- `country_code`
- `message`
- `database_ready`

Add root-admin endpoint:

- `POST /api/option/geoip/download`

The download endpoint uses the currently saved `geoip.download_url`, `geoip.database_path`, and optional `geoip.maxmind_license_key`. It returns success metadata with the final path and detected database filename.

## Frontend Settings

Add a new section under `System Settings -> Security -> GeoIP Access Restriction`.

The page mirrors the provided screenshot:

- Mode radio cards in a responsive two-column layout.
- Database path input.
- Download URL input.
- MaxMind License Key input with blank value after reload.
- Immediate download database button.
- Popup message textarea.
- Private and loopback IP bypass switch.
- Blocked country/region multi-select, defaulting to `China / China (CN)`.
- Save button in the section action area.

Country selection uses ISO 3166-1 alpha-2 codes. The component should allow searching by localized label, English name, or country code, and store only the country codes.

## Frontend Homepage Popup

The default frontend calls `GET /api/geoip/status` during public app startup.

- `homepage_notice`: show dismissible dialog.
- `homepage_block` and `homepage_block_api_reject`: show non-dismissible dialog.
- `off` or unblocked: show nothing.
- `full_reject`: backend rejects before frontend renders.

The dialog uses the configured message and does not expose implementation details such as country database path or source URL.

## Testing

Backend tests:

- GeoIP decision logic for off, notice, homepage block, API reject, and full reject modes.
- Private and loopback bypass behavior.
- Country code normalization and validation.
- Archive extraction picks the first valid `.mmdb` file and rejects archives without one.
- Middleware returns `403` for blocked API/full-reject cases and allows fail-open when the database is unavailable.

Frontend tests/type checks:

- Settings defaults parse correctly from system options.
- Saving serializes blocked countries as JSON.
- Popup behavior maps modes to dismissible/non-dismissible UI.

Manual verification:

- `go test ./...`
- `bun run typecheck` in `web/default`
- `bun run build` in `web/default`
- Start backend/frontend and verify the GeoIP settings section renders, saves, and download failures show a useful error.

## Sources

- P3TERX/GeoLite.mmdb: https://github.com/P3TERX/GeoLite.mmdb
- Loyalsoldier/geoip: https://github.com/Loyalsoldier/geoip
- wp-statistics/GeoLite2-Country: https://github.com/wp-statistics/GeoLite2-Country
