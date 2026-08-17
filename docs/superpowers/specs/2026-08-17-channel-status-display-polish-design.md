# Channel Status Display Polish Design

## Goal

Simplify the channel-status presentation by removing the cross-monitor overall
status judgment, stop rendering artificial missing timeline cells, and let an
administrator assign an optional hexadecimal color to each monitor's public
name.

This change keeps the operational, degraded, failed, and unknown state of each
individual monitor and model. It removes only the aggregate status calculated
across all published monitors.

## Confirmed Behavior

1. The signed-in channel-status summary shows the last update time and the four
   status counts, but no overall-status label or badge.
2. The public model-square summary keeps its channel-status heading, last update
   time, status counts, monitor rows, and details action, but no aggregate status
   badge.
3. A monitor card renders only real primary-model checks returned by the API,
   up to the existing 60-point limit. The available points expand evenly across
   the complete timeline width.
4. When a monitor has no recent check points, the timeline area displays a
   localized no-checks empty message instead of 60 neutral placeholders.
5. Every monitor may define an optional public-name color using exactly the
   `#RRGGBB` format. An empty value uses the normal theme foreground color.

## Data Model and Migration

Add `NameColor string` to `model.ChannelMonitor` with JSON name `name_color`
and a portable `varchar(7)`-equivalent GORM size. The existing `AutoMigrate`
registration for `ChannelMonitor` adds the column on SQLite, MySQL, and
PostgreSQL without dialect-specific SQL.

Existing rows receive an empty value and therefore preserve the current themed
text color.

`model.UpdateChannelMonitor` must include `name_color` in its explicit update
map. Lease ownership, scheduling timestamps, and runtime state remain outside
this display-only update.

## Input Contract and Validation

`service.ChannelMonitorInput` exposes `name_color` as `*string` rather than a
plain string:

- omitted during an update: preserve the current value, which keeps older API
  clients compatible;
- an explicit empty string: clear the custom color;
- a non-empty value: trim whitespace, require exactly `#RRGGBB`, and normalize
  hexadecimal letters to uppercase before storage.

Both backend normalization and the React Hook Form Zod schema enforce the same
contract. Invalid values return a field-level validation error and are never
persisted.

The administrator list/get DTO, create/update payloads, signed-in status item,
status detail, and public summary monitor include `name_color`.

## Overall Status API Removal

Remove `overall_status` from:

- `ChannelMonitorStatusResponse`;
- `ChannelMonitorPublicSummary`;
- the matching frontend Zod response schemas and fixtures.

Stop calling `channelMonitorOverallStatus` in the signed-in and public summary
builders. Status counts continue to be calculated because they remain visible.

`ChannelMonitorStatusDetail.status` remains unchanged. It represents one
monitor's combined primary and additional model state and is used inside that
monitor's detail drawer, not the removed cross-monitor judgment.

## Public Name Color Flow

The administrator form places an optional public-name color control in the
Identity section beside the naming fields. It contains:

- a familiar color swatch input;
- a text input for the exact hexadecimal code;
- an icon button with a tooltip to restore the theme default.

The swatch and text field update the same form value. The swatch uses a neutral
fallback only while the stored value is empty; it must not silently submit that
fallback. The public name is shown as a live preview in the form.

The configured color is applied through a dynamic inline `color` style only
when the value is non-empty. It is used for:

- the public name in the administrator monitor list;
- the monitor title on the signed-in channel-status card;
- the monitor title in the public model-square status summary;
- the monitor title in the status detail drawer.

All surrounding text, provider labels, status badges, and model names continue
to use theme colors. The application does not rewrite the administrator's RGB
value based on light or dark mode.

## Timeline Rendering

The backend timeline query remains unchanged: it returns at most 60 real
primary-model check rows from the preceding hour, newest first.

The monitor card continues to reverse those points for oldest-to-newest visual
order, but removes the fixed 60-slot padding. When points exist, the grid column
count equals the number of rendered points so every real point receives an
equal share of the available width. Each point keeps its status color, timestamp
tooltip, and accessible label.

When no point exists, the component renders a bounded, neutral empty area with
localized text. It does not manufacture unknown or failed checks.

## Internationalization

New user-visible labels, descriptions, reset tooltip text, validation text, and
the timeline empty message use `t(...)` keys. Locale changes are applied through
the repository translation script and synchronized for English, Simplified
Chinese, Traditional Chinese, French, Japanese, Russian, and Vietnamese.

## Testing

Backend behavior tests cover:

- valid, empty, omitted, and malformed `name_color` normalization;
- preserving the current color when an older update payload omits the field;
- returning the color from administrator, signed-in, detail, and public APIs;
- summary payloads no longer exposing or computing cross-monitor
  `overall_status`;
- database create/update behavior through GORM-compatible fixtures.

Frontend behavior tests cover:

- form schema acceptance and rejection for the exact hexadecimal contract;
- editing, clearing, swatch synchronization, and reset behavior;
- applying the configured color in every public-name presentation;
- rendering exactly the number of real timeline points and expanding them
  across the row;
- rendering the localized empty timeline state when no points exist;
- signed-in and public summaries no longer rendering an aggregate status badge;
- frontend response schemas accepting the revised API payloads.

Verification includes affected Go tests, affected frontend tests, the full Go
test suite, frontend type checking, lint on changed files, the complete frontend
test suite, production builds, and `git diff --check`.

## Out of Scope

- Changing probe scheduling, status classification, availability formulas, or
  retention rules.
- Changing individual monitor or model status badges.
- Automatically adjusting a configured name color for contrast.
- Adding colors to internal names, models, providers, or status timeline cells.
