# Log Reasoning Effort and Cache Hit Design

## Goal

Add reasoning effort and cache hit percentage to the usage log list and log details. Both fields are visible to users and administrators. Existing logs must be supported without a database migration.

## Scope

- Add a reasoning effort column to the desktop usage log list.
- Add a cache hit percentage column to the desktop usage log list.
- Show both values in mobile usage log cards.
- Add dedicated reasoning effort and cache hit sections to the log details dialog.
- Add translations for every new user-facing label in all supported default frontend locales.
- Preserve existing billing and token accounting behavior.

Filtering, sorting, database schema changes, and changes to upstream usage parsing are outside this scope.

## Data Sources

The existing log payload already contains the required values:

- `log.prompt_tokens`: provider-reported prompt or uncached input tokens, depending on usage semantics.
- `other.cache_tokens`: cache read tokens.
- `other.cache_write_tokens`: normalized cache creation total for newer logs.
- `other.cache_creation_tokens_5m` and `other.cache_creation_tokens_1h`: split Claude cache creation tokens.
- `other.cache_creation_tokens`: legacy or unsplit cache creation tokens.
- `other.usage_semantic === "anthropic"` or `other.claude === true`: identifies Claude/Anthropic token semantics.
- `other.reasoning_effort`: upstream reasoning effort value.

Reasoning effort is not treated as a closed enum. The UI displays the exact stored value so that current and future provider-specific values remain compatible. Missing values display `-`.

## Cache Write Normalization

The frontend derives cache write tokens without double-counting:

1. Use `other.cache_write_tokens` when present.
2. Otherwise, if either split field is present, use `cache_creation_tokens_5m + cache_creation_tokens_1h`.
3. Otherwise, use `other.cache_creation_tokens`.
4. Missing, negative, or non-finite values are treated as zero.

## Cache Hit Percentage

OpenAI-compatible usage reports cached tokens as part of input tokens:

```text
cache hit percentage = cache read tokens / prompt tokens * 100
```

Claude/Anthropic usage reports ordinary input, cache reads, and cache creation separately:

```text
total input = prompt tokens + cache read tokens + cache write tokens
cache hit percentage = cache read tokens / total input * 100
```

Claude semantics are selected when `usage_semantic` is `anthropic` or the legacy `claude` flag is true. All other logs use OpenAI-compatible semantics.

If the denominator is zero, the result is `0.000%`. The displayed value always uses exactly three decimal places. The calculation is clamped to the range from 0% through 100% to tolerate malformed historical data.

## User Interface

### Desktop Log List

Add two columns to common usage logs:

- Reasoning Effort: the raw stored value, or `-` when absent.
- Cache Hit: the calculated percentage, always rendered with three decimal places.

The columns are available to both users and administrators and participate in the existing column visibility persistence mechanism. They do not expose administrator-only data.

### Mobile Log Cards

Add compact rows for reasoning effort and cache hit percentage. The values and fallback behavior match the desktop table.

### Details Dialog

Replace the ungrouped reasoning effort row with a dedicated Reasoning Effort section. Display the raw value using the existing status badge treatment. Known values may retain their existing colors; unknown values use the default style without altering their text.

Add a Cache Hit section containing:

- Cache hit percentage.
- Cache read tokens.
- Total input tokens used as the denominator.

The existing Token Breakdown section remains unchanged and continues to show cache read and cache write token counts.

## Compatibility

- Historical OpenAI logs calculate from `prompt_tokens` and `cache_tokens`.
- Historical Claude logs are detected through either `usage_semantic` or `claude`.
- Historical Claude logs without `cache_write_tokens` use split cache creation fields or the legacy unsplit field.
- Logs without cache data display `0.000%`.
- Existing log API responses and database rows remain unchanged.

## Testing

Create frontend unit tests for a pure cache metric function before implementation. Cover:

- OpenAI input that includes cache reads.
- Claude input with separate cache reads and cache writes.
- Claude split 5-minute and 1-hour cache writes without double-counting.
- Legacy Claude cache creation fallback.
- Legacy `claude` semantic detection.
- Zero input and zero cache.
- Malformed negative or non-finite values.
- Percentage formatting with exactly three decimal places.

After implementation, run the focused unit tests, frontend typecheck, lint for changed files, i18n synchronization, and the production frontend build.

## Files Expected to Change

- Usage log formatting or metric helper under `web/default/src/features/usage-logs/lib/`.
- Common log desktop columns.
- Mobile usage log card.
- Details dialog.
- Usage log types if normalized fields are missing.
- All six frontend locale JSON files through the project i18n script.
- Focused frontend unit tests.
