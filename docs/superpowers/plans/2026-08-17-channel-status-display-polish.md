# Channel Status Display Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove cross-monitor aggregate status, render only real channel-check timeline points, and let administrators configure an optional public monitor-name color.

**Architecture:** Persist an optional normalized `#RRGGBB` color on each `ChannelMonitor`, carry it through the existing admin and public DTOs, and keep update omission distinct from an explicit clear by using a pointer input. The status summary APIs retain counts but drop `overall_status`; React schemas and components mirror that contract, render dynamic name color only when configured, and replace padded timeline slots with an empty state or an equal-width grid of real checks.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL, React 19, TypeScript, React Hook Form, Zod, Base UI, Tailwind CSS, Bun tests, i18next.

## Global Constraints

- Work only in `E:/github/new-api/.worktrees/channel-status-display-polish` on `codex/channel-status-display-polish`.
- Do not use subagents; execute this plan inline with `superpowers:executing-plans`.
- Preserve unrelated changes in the root checkout, especially `.gitignore` and `.playwright-cli/`.
- Write every behavior test first, run it, and confirm the expected failure before changing production code.
- All database behavior must remain compatible with SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6.
- Do not edit `web/src/i18n/locales/*.json` manually. Locale writes must go through `web/scripts/add-missing-keys.mjs`, followed by `bun run i18n:sync`.
- Preserve all new-api and QuantumNous branding, metadata, attributions, and copyright headers.
- Use `apply_patch` for manual file edits.

---

### Task 1: Persist and Normalize the Optional Monitor Name Color

**Files:**
- Modify: `model/channel_monitor.go`
- Modify: `model/channel_monitor_test.go`
- Modify: `service/channel_monitor_config.go`
- Modify: `service/channel_monitor_config_test.go`

**Interfaces:**
- Produces: `ChannelMonitor.NameColor string` serialized as `name_color` and stored with `gorm:"size:7"`.
- Produces: `ChannelMonitorInput.NameColor *string` serialized as `name_color`.
- Produces: `NormalizeChannelMonitorInput` semantics where omitted preserves the current value, empty clears it, and valid input is trimmed and uppercased.

- [ ] **Step 1: Add a failing model persistence regression test**

Extend the existing SQLite-backed create/update test so a monitor is created with `NameColor: "#1A2B3C"`, read back with that value, updated to `"#ABCDEF"`, and read back again. This test must fail before implementation because the field and explicit update-map entry do not exist.

- [ ] **Step 2: Run the model test and verify RED**

Run:

```powershell
go test ./model -run TestChannelMonitorCreateUpdateAndDelete -count=1
```

Expected: compilation failure for the missing `NameColor` field, or an assertion failure showing the update did not persist it.

- [ ] **Step 3: Implement portable persistence**

Add the field next to `PublicName`:

```go
NameColor string `json:"name_color" gorm:"size:7"`
```

Add `"name_color": monitor.NameColor` to the explicit `UpdateChannelMonitor` update map. Rely on the existing `AutoMigrate(&ChannelMonitor{})`; do not add dialect-specific SQL.

- [ ] **Step 4: Run the model test and verify GREEN**

Run the same `go test` command and require it to pass.

- [ ] **Step 5: Add failing normalization table tests**

Add explicit cases to `service/channel_monitor_config_test.go` covering:

```go
valid := "  #1a2b3c  " // result must be #1A2B3C
clear := ""            // result must be empty
omitted := (*string)(nil) // existing #ABCDEF must remain unchanged
invalid := []string{"112233", "#123", "#12345G", "#1234567"}
```

Use a complete valid monitor input fixture for every case. Each invalid value must return an error and no monitor.

- [ ] **Step 6: Run the normalization test and verify RED**

Run:

```powershell
go test ./service -run TestNormalizeChannelMonitorInputNameColor -count=1
```

Expected: compilation failure because `ChannelMonitorInput.NameColor` and `ChannelMonitor.NameColor` are missing from normalization.

- [ ] **Step 7: Implement minimal normalization**

Add `NameColor *string` to the input DTO. In `NormalizeChannelMonitorInput`, only mutate `monitor.NameColor` when the pointer is non-nil. Trim the value, accept empty as clear, otherwise require exact `^#[0-9A-Fa-f]{6}$`, then store `strings.ToUpper(value)`. Return a stable validation error such as `channel monitor name color must use #RRGGBB format` for malformed input.

- [ ] **Step 8: Run targeted backend tests and commit**

Run:

```powershell
go test ./model ./service -run 'Test(ChannelMonitorCreateUpdateAndDelete|NormalizeChannelMonitorInputNameColor)' -count=1
git add model/channel_monitor.go model/channel_monitor_test.go service/channel_monitor_config.go service/channel_monitor_config_test.go
git commit -m "feat: add channel monitor name colors"
```

---

### Task 2: Revise Channel Status API Contracts

**Files:**
- Modify: `service/channel_monitor_query.go`
- Modify: `service/channel_monitor_query_test.go`
- Modify: `controller/channel_monitor_test.go`

**Interfaces:**
- Consumes: `ChannelMonitor.NameColor string` from Task 1.
- Produces: `name_color` on signed-in status items, monitor details, and public-summary monitor rows.
- Produces: summary responses with `last_updated_at`, `counts`, and items/monitors, but no `overall_status`.
- Preserves: `ChannelMonitorStatusDetail.Status`, still calculated across models within one monitor.

- [ ] **Step 1: Add failing service and controller contract assertions**

Update the existing public/signed-in query fixture to persist a distinct `NameColor`, assert it on `publicSummary.Monitors[0]`, `signedIn.Items[0]`, and `GetPublishedChannelMonitorDetail`, and remove assertions against summary `OverallStatus`. Marshal the two summary DTOs with `common.Marshal` and assert their JSON does not contain `"overall_status"`. Keep the detail-status assertion so removal of the per-monitor aggregation would fail the test.

In the existing status/public-summary controller tests, decode the JSON response as a generic map through `common.Unmarshal` and assert summary objects do not contain `overall_status`, while a monitor object contains the configured `name_color`.

- [ ] **Step 2: Run the service and controller tests and verify RED**

Run:

```powershell
go test ./service -run TestChannelMonitorPublicAndSignedInQueriesFilterAndIsolateSensitiveFields -count=1
go test ./controller -run ChannelMonitor -count=1
```

Expected: compilation failure for missing `NameColor` DTO fields or a failed JSON assertion because summary `overall_status` is still present.

- [ ] **Step 3: Implement the revised service DTOs and queries**

Add `NameColor string \`json:"name_color"\`` to `ChannelMonitorStatusItem`, `ChannelMonitorStatusDetail`, `ChannelMonitorPublicMonitor`, and `channelMonitorPublishedRow`. Include `name_color` in both published-monitor `Select(...)` calls and copy it into all three response shapes.

Delete `OverallStatus` from `ChannelMonitorStatusResponse` and `ChannelMonitorPublicSummary`; remove their default initialization and the two final calls to `channelMonitorOverallStatus`. Leave the helper and the detail call intact.

- [ ] **Step 4: Run service and controller tests and verify GREEN**

Run:

```powershell
go test ./controller -run ChannelMonitor -count=1
```

Require both commands to pass with the revised JSON contract.

- [ ] **Step 5: Run the affected backend packages and commit**

Run:

```powershell
go test ./model ./service ./controller -run ChannelMonitor -count=1
git add service/channel_monitor_query.go service/channel_monitor_query_test.go controller/channel_monitor_test.go
git commit -m "refactor: simplify channel status summaries"
```

---

### Task 3: Add Frontend Schema and Administrator Color Controls

**Files:**
- Modify: `web/src/features/channel-monitors/types.ts`
- Modify: `web/src/features/channel-monitors/lib/schema.ts`
- Modify: `web/src/features/channel-monitors/lib/__tests__/schema.test.ts`
- Modify: `web/src/features/channel-monitors/components/channel-monitor-form-drawer.tsx`
- Modify: `web/src/features/channel-monitors/components/channel-monitor-table.tsx`
- Modify: `web/src/features/channel-monitors/components/__tests__/management.test.tsx`

**Interfaces:**
- Consumes: backend `name_color` field from Tasks 1-2.
- Produces: `name_color: string` in monitor/form types and create/update payloads.
- Produces: optional color text/swatch/reset form control and colored admin-list public-name preview.

- [ ] **Step 1: Inspect project shadcn context and rules**

From `web/`, run the bundled Bun executable with:

```powershell
& 'C:\Users\Administrator\.cache\codex-runtimes\codex-bun-npm\node_modules\.pnpm\@oven+bun-windows-x64@1.3.14\node_modules\@oven\bun-windows-x64\bin\bun.exe' x shadcn@latest info --json
```

Read `.agents/skills/shadcn-ui/vendor/shadcn/rules/forms.md`, `icons.md`, and `styling.md` before editing the form.

- [ ] **Step 2: Add failing Zod schema tests**

Extend the valid fixture with `name_color: ''`. Add exact cases that accept `''`, `#1A2B3C`, and lowercase `#abcdef`, while rejecting values without `#`, short/long values, and non-hex characters. Assert the schema normalizes accepted non-empty values to uppercase if normalization is implemented in Zod; otherwise assert the payload normalization at the form boundary.

- [ ] **Step 3: Run schema tests and verify RED**

Run from `web/`:

```powershell
& $bun test src/features/channel-monitors/lib/__tests__/schema.test.ts --isolate
```

Expected: failing acceptance/rejection assertions because `name_color` is absent.

- [ ] **Step 4: Implement frontend types and schema**

Add `name_color: string` to `ChannelMonitor`, `ChannelMonitorFormValues`, defaults, edit reset values, and API request payloads. Define the Zod field as a trimmed string that is empty or matches `/^#[0-9A-Fa-f]{6}$/`, then transform each non-empty accepted value to uppercase.

- [ ] **Step 5: Add failing administrator interaction tests**

Extend `management.test.tsx` to verify:

- an existing color is loaded into the text input and live public-name preview;
- typing a valid hex value updates both preview style and the swatch value;
- changing the swatch updates the text field;
- the reset icon clears the field and removes the inline preview color;
- the table's public name receives the configured inline color while the internal name does not.

Use accessible labels and user-visible behavior rather than private form state.

- [ ] **Step 6: Run management tests and verify RED**

Run:

```powershell
& $bun test src/features/channel-monitors/components/__tests__/management.test.tsx --isolate
```

Expected: missing control/label/style assertions fail.

- [ ] **Step 7: Implement the form and admin preview**

In the Identity section, add one `FormField` for `name_color` containing:

- `Input type="color"` as the familiar swatch, using a neutral fallback only for the HTML input value;
- a text `Input` with `#RRGGBB` placeholder and field error message;
- a reset-to-theme-default icon button with a tooltip and accessible label;
- a live public-name preview whose inline `style` is `{ color: field.value }` only when non-empty.

Use an existing Lucide reset/rotate icon, fixed icon-button dimensions, and no nested cards. Apply the same conditional inline style to the public-name line in `ChannelMonitorTable`.

- [ ] **Step 8: Run targeted frontend tests and commit**

Run:

```powershell
& $bun test src/features/channel-monitors/lib/__tests__/schema.test.ts src/features/channel-monitors/components/__tests__/management.test.tsx --isolate
git add web/src/features/channel-monitors
git commit -m "feat: configure channel monitor name colors"
```

---

### Task 4: Remove Aggregate Badges and Render Real Timeline Points

**Files:**
- Modify: `web/src/features/channel-status/types.ts`
- Modify: `web/src/features/channel-status/components/status-summary.tsx`
- Modify: `web/src/features/channel-status/components/public-status-summary.tsx`
- Modify: `web/src/features/channel-status/components/monitor-status-card.tsx`
- Modify: `web/src/features/channel-status/components/monitor-status-details.tsx`
- Modify: `web/src/features/channel-status/components/__tests__/status-page.test.tsx`
- Modify: `web/src/features/channel-status/components/__tests__/public-summary.test.tsx`

**Interfaces:**
- Consumes: revised API contracts and `name_color` from Task 2.
- Produces: no aggregate status indicator in signed-in/public summaries.
- Produces: timeline with exactly the real point count, oldest-to-newest, evenly stretched across the row.
- Produces: localized empty timeline state when there are no points.

- [ ] **Step 1: Add failing status-page tests**

Remove `overall_status` from fixtures and add `name_color`. Assert:

- the summary region has no aggregate status indicator while monitor indicators remain;
- a two-point timeline renders exactly two `[data-slot="timeline-point"]` elements in oldest-to-newest order;
- the timeline grid uses two equal columns rather than the fixed 60-column class;
- an empty timeline renders `[data-slot="timeline-empty"]` with the translated no-checks message and zero timeline-point elements;
- the monitor card and detail title apply the configured inline color.

- [ ] **Step 2: Run status-page tests and verify RED**

Run:

```powershell
& $bun test src/features/channel-status/components/__tests__/status-page.test.tsx --isolate
```

Expected: schema/fixture mismatch, 60 padded points, missing empty state, or missing color styles.

- [ ] **Step 3: Add failing public-summary tests**

Remove `overall_status` from the fixture, add distinct `name_color` values, and assert the summary has no aggregate indicator while every monitor keeps its own indicator and configured public-name color.

- [ ] **Step 4: Run public-summary tests and verify RED**

Run:

```powershell
& $bun test src/features/channel-status/components/__tests__/public-summary.test.tsx --isolate
```

Expected: missing `name_color` schema support or the aggregate indicator is still rendered.

- [ ] **Step 5: Implement schemas and summary rendering**

Add `name_color` to all monitor/detail Zod schemas and inferred types. Remove `overall_status` from signed-in and public summary schemas. Remove only the summary-level `ChannelStatusIndicator` calls; keep count rendering and all monitor/model indicators. Apply conditional inline color to public names in the public summary.

- [ ] **Step 6: Implement real-point timeline and detail color**

Keep `slice(0, 60).reverse()` but remove `TIMELINE_SLOTS`, `firstPointSlot`, nullable points, and the `missing` timeline status. When points exist, set the grid's inline `gridTemplateColumns` to `repeat(${recentPoints.length}, minmax(0, 1fr))` and map the points directly. When empty, render a bounded neutral container with `data-slot="timeline-empty"` and `t('No recent checks')`. Apply the configured name color to the card title and detail public-name title only.

- [ ] **Step 7: Run all channel-status frontend tests and commit**

Run:

```powershell
& $bun test src/features/channel-status/components/__tests__/status-page.test.tsx src/features/channel-status/components/__tests__/public-summary.test.tsx src/features/channel-status/hooks/__tests__/refresh.test.tsx --isolate
git add web/src/features/channel-status
git commit -m "fix: clarify channel status displays"
```

---

### Task 5: Add and Synchronize Localized UI Copy

**Files:**
- Create temporarily, then delete after applying translations: `web/scripts/add-missing-keys.mjs`
- Modify through script only: `web/src/i18n/locales/en.json`
- Modify through script only: `web/src/i18n/locales/zh.json`
- Modify through script only: `web/src/i18n/locales/zh-TW.json`
- Modify through script only: `web/src/i18n/locales/fr.json`
- Modify through script only: `web/src/i18n/locales/ja.json`
- Modify through script only: `web/src/i18n/locales/ru.json`
- Modify through script only: `web/src/i18n/locales/vi.json`

**Interfaces:**
- Consumes: exact `t(...)` keys introduced by Tasks 3-4.
- Produces: complete translations for color label/help/reset/validation/preview and timeline empty copy in all seven locales.

- [ ] **Step 1: Run the sync report before locale changes**

Run from `web/`:

```powershell
& $bun run i18n:sync
```

Read `src/i18n/locales/_reports/_sync-report.json` and identify only keys introduced by this feature.

- [ ] **Step 2: Populate the sanctioned translation script**

Create `web/scripts/add-missing-keys.mjs` using the exact structure required by the `i18n-translate` skill, then populate it with each new English key and natural, compact values for `en`, `zh`, `zh-TW`, `fr`, `ja`, `ru`, and `vi`. Preserve interpolation placeholders exactly. Do not write locale JSON with `apply_patch`.

- [ ] **Step 3: Apply and synchronize translations**

Run:

```powershell
node scripts/add-missing-keys.mjs
& $bun run i18n:sync
```

Delete the temporary script with `apply_patch`. Verify the report has no missing keys caused by this change and that all locale files remain valid JSON.

- [ ] **Step 4: Run affected frontend tests and commit**

Run:

```powershell
& $bun test src/features/channel-monitors src/features/channel-status --isolate
git add web/src/i18n/locales
git commit -m "i18n: localize channel monitor display settings"
```

---

### Task 6: Full Verification and Local Acceptance Server

**Files:**
- Verify all changed files.
- Do not alter production behavior unless a verification failure exposes a regression caused by this work; fix such regressions through a new red-green cycle.

**Interfaces:**
- Produces: a verified branch and a running local server URL for user acceptance testing.

- [ ] **Step 1: Run backend verification**

Run:

```powershell
go test ./...
go build ./...
```

- [ ] **Step 2: Run frontend verification**

Run from `web/`:

```powershell
& $bun test --isolate
& $bun run typecheck
& $bun run lint -- src/features/channel-monitors src/features/channel-status
& $bun run build
```

If the lint script does not accept paths after `--`, invoke the installed `oxlint` binary directly against only the changed TypeScript/TSX files.

- [ ] **Step 3: Run repository hygiene checks**

Run:

```powershell
git diff --check
git status --short
git log --oneline --decorate -8
```

Confirm no root-checkout files or unrelated generated files were changed.

- [ ] **Step 4: Perform browser acceptance checks**

Start the local backend/frontend using the repository's established development command and an unused port. Verify at desktop and mobile widths:

- admin create/edit color controls synchronize and reset correctly;
- admin list, signed-in status card, public summary, and detail title use the configured color;
- status counts remain visible without an aggregate badge;
- timelines show only real points and empty monitors show the localized empty state;
- light and dark themes remain legible and no text or controls overlap.

- [ ] **Step 5: Report the acceptance URL and exact verification results**

Keep the local service running for the user. Report the URL, branch name, commits, test/build results, and any residual limitations without claiming completion before the commands above have passed.
