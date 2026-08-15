# Restore Log TPS, API Key Dark Actions, and First-Visit Theme Design

## Goal

Restore the desktop usage-log throughput display, improve the contrast of the API key page's blue action controls in dark mode without changing global outline buttons, and make the existing `simple-large` preset the first-visit default while preserving every later user choice.

## Usage Log Throughput

The backend payload and shared `StreamTpsCell` still support throughput. The desktop common-log column currently suppresses it with `showTokensPerSecond={false}` and no longer calculates the value.

The desktop stream column will calculate throughput with the same rule already used by the mobile card:

```text
completion_tokens / use_time
```

The value is available only when both fields are positive. `StreamTpsCell` will continue rounding the displayed result to an integer and will show an em dash when throughput is unavailable. The stream column will grow from 50px to 72px so its two compact lines fit without restoring the former excessive width.

## API Key Action Theme

Add API-key-specific semantic theme tokens for action border/text, surface, and hover foreground. The light values will match the current outline-action appearance. The dark values will use a brighter cyan-blue border/text on a deliberately darker blue-neutral surface so the controls remain crisp against the dark table background and satisfy readable contrast.

Register the tokens with Tailwind and add one `apiKeyAction` button variant. Apply that variant only to:

- the fixed-width Copy button beside a masked API key;
- the compact group selector used in the API key table;
- the One-click import to CC Switch button.

Non-compact group selectors and every other outline button remain unchanged. Existing fixed sizing and disabled, focus, popup-open, and auto-group behavior remain intact.

## First-Visit Theme Default

The existing `simple-large` preset will become `DEFAULT_THEME_CUSTOMIZATION.preset`. This changes only the preset axis; theme mode, font override, radius, density, and content layout keep their current defaults.

The provider must distinguish the selected default preset from the CSS base preset. It will remove `data-theme-preset` only for the literal `default` palette, while `simple-large` always writes `data-theme-preset="simple-large"` so its CSS is active.

The selected preset will always be persisted in the existing `theme_preset` browser cookie, including explicit selections of `default` and `simple-large`. On a browser with no valid preset cookie, the provider starts with `simple-large` and persists it. After the user selects another preset, that exact value is restored on later visits. Clearing browser site data removes the preference and causes the first-visit default to apply again.

## Testing

- Update the stream-cell regression test so visible integer `t/s` output is the protected default behavior, while unavailable throughput still renders an em dash.
- Add a common-log column test that verifies the desktop stream column derives throughput from completion tokens and request duration and retains the 72px width contract.
- Update API key component tests to require the dedicated semantic action classes on the Copy and compact group controls.
- Add a focused row-action test for the CC Switch control's dedicated variant.
- Add provider tests for first-visit `simple-large`, explicit `default` persistence, and remount restoration.
- Run focused tests, the affected frontend test set, type checking, focused lint, formatting checks, and a production build.
- Verify the API key page in light and dark modes and the usage-log page at desktop width with Playwright screenshots.

## Out of Scope

- No backend or database changes.
- No changes to throughput calculation semantics beyond restoring the established mobile rule on desktop.
- No global outline-button color changes.
- No new user-facing copy or translation keys.
