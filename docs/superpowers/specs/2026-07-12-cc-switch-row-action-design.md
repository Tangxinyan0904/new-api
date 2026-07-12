# CC Switch Row Action Design

## Goal

Make the per-key CC Switch import entry directly visible while keeping the existing API-key action controls compact and unchanged on their first row.

## Layout

The existing row-action component becomes a two-row vertical group:

- Top row: keep the current enable/disable button, edit button, and overflow menu in their existing order and visual treatment.
- Bottom row: add one full-width outline button with the existing `ArrowRightLeft` icon and the label `One-click import to CC Switch`.

The shared row-action component is used by both the desktop table and mobile cards, so the same two-row layout applies to both views. The button width is determined by the action group and must not be placed in the API Key cell or the global toolbar.

## Behavior

The new button reuses the current CC Switch flow:

1. Resolve the selected row's full API key with `resolveRealKey`.
2. Disable the button and show the existing loading spinner while the key is resolving.
3. Store the resolved key and current row in the existing API-key provider.
4. Open the existing `cc-switch` dialog.

Remove the old CC Switch item from the overflow menu. Preserve Copy Key, Copy Connection Info, optional Chat presets, and Delete. Render menu separators conditionally so removing CC Switch does not leave adjacent separators when Chat presets are absent.

The CC Switch dialog, provider state model, URL generation, model selection, and backend APIs remain unchanged.

## Internationalization

Use a new key so the row button label does not change the existing dialog title:

| Locale | Value |
| --- | --- |
| en | One-click import to CC Switch |
| zh | 一键导入CC Switch |
| zh-TW | 一鍵匯入CC Switch |
| fr | Importer en un clic vers CC Switch |
| ja | ワンクリックで CC Switch にインポート |
| ru | Импорт в CC Switch одним нажатием |
| vi | Nhập vào CC Switch bằng một lần nhấp |

Locale writes must go through the project's `add-missing-keys.mjs` workflow, followed by `bun run i18n:sync`. Add the English source key to `src/i18n/static-keys.ts`.

## Verification

This is an approved layout relocation of an existing action, so no new component test is required. Verify:

- The button is visible below the unchanged top action row on desktop and mobile.
- The overflow menu no longer contains CC Switch and has no duplicate separators.
- The loading state prevents duplicate imports while the real key resolves.
- Clicking the button opens the existing dialog for the selected API key.
- i18n sync, focused lint, formatting, TypeScript checking, and the production build all pass.
- The local frontend on port 3001 hot-reloads and its proxied API remains available.
