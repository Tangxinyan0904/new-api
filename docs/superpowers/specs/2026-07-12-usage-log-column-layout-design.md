# Usage Log Column Layout Design

## Goal

Make the desktop common-usage-log table denser by reducing the Stream column's default width and placing Cache Hit immediately after Tokens.

## Scope

- Keep the existing Stream / Non-stream label, TPS value, and stream-error tooltip.
- Change the Stream column's default size from TanStack Table's 150 px default to 50 px.
- Change the desktop column order from `Tokens -> Cost -> Cache Hit` to `Tokens -> Cache Hit -> Cost`.
- Leave the mobile log card and log-details dialog unchanged.
- Do not change cache-hit calculation, formatting, highlighting, or visibility rules.

## Implementation

Update the common usage-log column definitions in `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx`:

1. Set `size: 50` on the `is_stream` column.
2. Move the existing `cache_hit` column definition directly after the `prompt_tokens` column.
3. Keep the existing Cost and Timing definitions otherwise unchanged.

## Verification

- Run the existing cache-metrics regression test.
- Run focused lint and formatting checks for the changed TSX file.
- Run the default frontend TypeScript check and production build.
- Inspect the resulting diff to confirm only column width and order changed.

No new automated test will be added because the requested behavior is static table layout configuration; a source-order/constant assertion would lock in implementation details without protecting cache calculation or another user-facing data contract.
