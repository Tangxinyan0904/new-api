# Wallet Special Ratio Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let administrators select special group-ratio rules for display in a dedicated wallet card visible to every authenticated wallet user.

**Architecture:** Keep billing ratios unchanged and store wallet visibility in a separate nested boolean setting. Build a sanitized, deterministic user-facing projection in the service layer, expose it through an authenticated endpoint, then render it in an independently loaded wallet card whose availability drives the responsive wallet grid.

**Tech Stack:** Go 1.22+, Gin, project setting/config and `types.RWMap` APIs, React 19, TypeScript, React Query, React Hook Form, Zod, Base UI/Tailwind, Bun tests, testify.

---

## File Structure

Backend responsibilities:

- Modify `setting/ratio_setting/group_ratio.go`: own the new visibility map, parse/normalize/validate it, and provide copied snapshots with the related ratio maps.
- Create `setting/ratio_setting/group_ratio_wallet_display_test.go`: protect JSON normalization, cross-field validation, and replacement behavior.
- Modify `controller/option.go`: reject invalid direct writes before persistence.
- Modify `model/option.go`: route the typed visibility option through the explicit ratio-setting updater.
- Modify `service/group.go`: build the public projection from copied pricing snapshots.
- Create `service/group_wallet_ratio_test.go`: protect filtering, exact values, and ordering.
- Modify `controller/group.go`: return the projection through the standard API envelope.
- Modify `router/api-router.go`: register the endpoint inside the existing `UserAuth` group.
- Create `router/wallet_special_ratios_test.go`: protect the authentication boundary.

Frontend responsibilities:

- Create `web/default/src/features/system-settings/models/group-ratio-wallet-display.ts`: pure parsing, mutation, pruning, cross-field validation, and ordered save-update helpers.
- Create `web/default/src/features/system-settings/models/group-ratio-wallet-display.test.ts`: test those contracts without coupling to component internals.
- Modify `web/default/src/features/system-settings/models/group-ratio-visual-editor.tsx`: add the Wallet display checkbox and keep selections aligned during edit/delete operations.
- Modify `web/default/src/features/system-settings/models/group-ratio-form.tsx`: carry the field through visual and JSON modes.
- Modify `web/default/src/features/system-settings/models/ratio-settings-card.tsx`: add schema/default/dirty/save handling and enforce ratio-before-visibility writes.
- Modify `web/default/src/features/system-settings/billing/{index.tsx,section-registry.tsx}` and `web/default/src/features/system-settings/types.ts`: expose the setting to the form.
- Modify `web/default/src/features/wallet/{types.ts,api.ts}`: type and load the authenticated projection.
- Create `web/default/src/features/wallet/lib/special-ratios.ts`: pure view-state, row-label, and layout helpers.
- Create `web/default/src/features/wallet/lib/special-ratios.test.ts`: protect card state and responsive grid selection.
- Create `web/default/src/features/wallet/components/special-ratio-rules-card.tsx`: render loading, error/retry, success, and empty states.
- Modify `web/default/src/features/wallet/index.tsx`: mount the new card and apply the availability-driven grid.
- Modify all files under `web/default/src/i18n/locales/*.json`: add complete translations.

## Task 1: Persist and Validate Wallet Visibility Rules

**Files:**

- Modify: `setting/ratio_setting/group_ratio.go`
- Create: `setting/ratio_setting/group_ratio_wallet_display_test.go`
- Modify: `controller/option.go`
- Modify: `model/option.go`

- [x] **Step 1: Write failing setting tests**

Create `setting/ratio_setting/group_ratio_wallet_display_test.go`:

```go
package ratio_setting

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWalletDisplayRulesNormalizeAndReplace(t *testing.T) {
    originalRatios := GroupGroupRatio2JSONString()
    originalDisplay := GroupGroupRatioWalletDisplay2JSONString()
    t.Cleanup(func() {
        require.NoError(t, UpdateGroupGroupRatioByJSONString(originalRatios))
        require.NoError(t, UpdateGroupGroupRatioWalletDisplayByJSONString(originalDisplay))
    })

    require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"premium":0.3},"staff":{"default":0.8}}`))
    require.NoError(t, UpdateGroupGroupRatioWalletDisplayByJSONString(`{"vip":{"premium":true},"staff":{"default":false}}`))
    assert.JSONEq(t, `{"vip":{"premium":true}}`, GroupGroupRatioWalletDisplay2JSONString())

    require.NoError(t, UpdateGroupGroupRatioWalletDisplayByJSONString(`{}`))
    assert.JSONEq(t, `{}`, GroupGroupRatioWalletDisplay2JSONString())
}

func TestValidateWalletDisplayRulesAgainstSpecialRatios(t *testing.T) {
    originalRatios := GroupGroupRatio2JSONString()
    t.Cleanup(func() {
        require.NoError(t, UpdateGroupGroupRatioByJSONString(originalRatios))
    })

    require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"premium":0.3}}`))
    require.NoError(t, ValidateGroupGroupRatioWalletDisplay(`{"vip":{"premium":true}}`))
    require.ErrorContains(t, ValidateGroupGroupRatioWalletDisplay(`{"vip":{"missing":true}}`), "vip -> missing")
    require.ErrorContains(t, ValidateGroupGroupRatioWalletDisplay(`{"":{"premium":true}}`), "user group")
    require.Error(t, ValidateGroupGroupRatioWalletDisplay(`[]`))
}
```

- [x] **Step 2: Run the new test and verify the missing API failure**

Run:

```powershell
go test ./setting/ratio_setting -run 'TestWalletDisplay' -count=1
```

Expected: compilation fails because the four `GroupGroupRatioWalletDisplay...` functions do not exist.

- [x] **Step 3: Add the setting, normalization, validation, and snapshots**

In `setting/ratio_setting/group_ratio.go`, add the option constant, map, struct field, initialization, and these APIs. Use `common.Unmarshal`/`common.Marshal`; do not add new direct `encoding/json` calls.

```go
const GroupGroupRatioWalletDisplayOption = "group_ratio_setting.group_group_ratio_wallet_display"

var groupPricingSnapshotMutex sync.RWMutex
var groupGroupRatioWalletDisplayMap = types.NewRWMap[string, map[string]bool]()

type WalletRatioSettingsSnapshot struct {
    BaseRatios    map[string]float64
    SpecialRatios map[string]map[string]float64
    WalletDisplay map[string]map[string]bool
}

func parseGroupGroupRatioWalletDisplay(jsonStr string) (map[string]map[string]bool, error) {
    raw := make(map[string]map[string]bool)
    if err := common.UnmarshalJsonStr(jsonStr, &raw); err != nil {
        return nil, err
    }
    if raw == nil {
        return nil, errors.New("wallet display rules must be a JSON object")
    }

    normalized := make(map[string]map[string]bool)
    for userGroup, targets := range raw {
        if strings.TrimSpace(userGroup) == "" {
            return nil, errors.New("wallet display user group must not be empty")
        }
        for billingGroup, visible := range targets {
            if strings.TrimSpace(billingGroup) == "" {
                return nil, errors.New("wallet display billing group must not be empty")
            }
            if !visible {
                continue
            }
            if normalized[userGroup] == nil {
                normalized[userGroup] = make(map[string]bool)
            }
            normalized[userGroup][billingGroup] = true
        }
    }
    return normalized, nil
}

func ValidateGroupGroupRatioWalletDisplay(jsonStr string) error {
    display, err := parseGroupGroupRatioWalletDisplay(jsonStr)
    if err != nil {
        return err
    }
    special := groupGroupRatioMap.ReadAll()
    for userGroup, targets := range display {
        ratios, ok := special[userGroup]
        if !ok {
            return fmt.Errorf("wallet display rule %s has no special ratio group", userGroup)
        }
        for billingGroup := range targets {
            if _, ok := ratios[billingGroup]; !ok {
                return fmt.Errorf("wallet display rule %s -> %s has no special ratio", userGroup, billingGroup)
            }
        }
    }
    return nil
}

func UpdateGroupGroupRatioWalletDisplayByJSONString(jsonStr string) error {
    normalized, err := parseGroupGroupRatioWalletDisplay(jsonStr)
    if err != nil {
        return err
    }
    encoded, err := common.Marshal(normalized)
    if err != nil {
        return err
    }
    groupPricingSnapshotMutex.Lock()
    defer groupPricingSnapshotMutex.Unlock()
    return types.LoadFromJsonString(groupGroupRatioWalletDisplayMap, string(encoded))
}

func GroupGroupRatioWalletDisplay2JSONString() string {
    return groupGroupRatioWalletDisplayMap.MarshalJSONString()
}

func GetWalletRatioSettingsSnapshot() WalletRatioSettingsSnapshot {
    groupPricingSnapshotMutex.RLock()
    defer groupPricingSnapshotMutex.RUnlock()
    return WalletRatioSettingsSnapshot{
        BaseRatios:    copyFloatMap(groupRatioMap.ReadAll()),
        SpecialRatios: copyNestedFloatMap(groupGroupRatioMap.ReadAll()),
        WalletDisplay: copyNestedBoolMap(groupGroupRatioWalletDisplayMap.ReadAll()),
    }
}
```

Add small copy loops in the same file for `copyFloatMap`, `copyNestedFloatMap`, and `copyNestedBoolMap`; each allocates a new outer map and new inner maps. Add `GroupGroupRatioWalletDisplay` to `GroupRatioSetting` with JSON tag `group_group_ratio_wallet_display`, initialize it to `groupGroupRatioWalletDisplayMap`, and restore it in `GetGroupRatioSetting` when nil.

Wrap the bodies of `UpdateGroupRatioByJSONString` and `UpdateGroupGroupRatioByJSONString` with `groupPricingSnapshotMutex.Lock()`/`Unlock()` so the new snapshot cannot observe a half-applied in-process update.

- [x] **Step 4: Validate the typed option before persistence and use the explicit updater**

In `controller/option.go`, add this case to the existing validation switch before `model.UpdateOption`:

```go
case ratio_setting.GroupGroupRatioWalletDisplayOption:
    if err := ratio_setting.ValidateGroupGroupRatioWalletDisplay(option.Value.(string)); err != nil {
        common.ApiError(c, err)
        return
    }
```

In `model/option.go`, immediately after assigning `common.OptionMap[key] = value` and before `handleConfigUpdate`, add:

```go
if key == ratio_setting.GroupGroupRatioWalletDisplayOption {
    return ratio_setting.UpdateGroupGroupRatioWalletDisplayByJSONString(value)
}
```

This keeps runtime writes under the snapshot mutex while the registered config field still exports the same `RWMap` value.

- [x] **Step 5: Run focused setting tests**

Run:

```powershell
go test ./setting/ratio_setting -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the backend setting boundary**

```powershell
git add setting/ratio_setting/group_ratio.go setting/ratio_setting/group_ratio_wallet_display_test.go controller/option.go model/option.go
git commit -m "feat(billing): configure wallet-visible special ratios"
```

## Task 2: Build and Expose the Authenticated Rule Projection

**Files:**

- Modify: `service/group.go`
- Create: `service/group_wallet_ratio_test.go`
- Modify: `controller/group.go`
- Modify: `router/api-router.go`
- Create: `router/wallet_special_ratios_test.go`

- [ ] **Step 1: Write the failing projection test**

Create `service/group_wallet_ratio_test.go`:

```go
package service

import (
    "math"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestBuildWalletSpecialRatioRulesFiltersAndSorts(t *testing.T) {
    got := buildWalletSpecialRatioRules(
        map[string]float64{"default": 1, "premium": 0.5},
        map[string]map[string]float64{
            "vip":   {"premium": 0.3, "default": 0.8},
            "staff": {"premium": math.Inf(1)},
        },
        map[string]map[string]bool{
            "vip":     {"premium": true, "default": true, "missing": true},
            "staff":   {"premium": true},
            "orphaned": {"default": true},
        },
    )

    assert.Equal(t, []WalletSpecialRatioRule{
        {UserGroup: "vip", BillingGroup: "default", SpecialRatio: 0.8, BaseRatio: 1},
        {UserGroup: "vip", BillingGroup: "premium", SpecialRatio: 0.3, BaseRatio: 0.5},
    }, got)
}
```

- [ ] **Step 2: Run the service test and verify it fails**

Run:

```powershell
go test ./service -run TestBuildWalletSpecialRatioRulesFiltersAndSorts -count=1
```

Expected: compilation fails because `WalletSpecialRatioRule` and `buildWalletSpecialRatioRules` do not exist.

- [ ] **Step 3: Implement the deterministic service projection**

Append to `service/group.go`:

```go
type WalletSpecialRatioRule struct {
    UserGroup    string  `json:"user_group"`
    BillingGroup string  `json:"billing_group"`
    SpecialRatio float64 `json:"special_ratio"`
    BaseRatio    float64 `json:"base_ratio"`
}

func GetWalletSpecialRatioRules() []WalletSpecialRatioRule {
    snapshot := ratio_setting.GetWalletRatioSettingsSnapshot()
    return buildWalletSpecialRatioRules(
        snapshot.BaseRatios,
        snapshot.SpecialRatios,
        snapshot.WalletDisplay,
    )
}

func buildWalletSpecialRatioRules(
    base map[string]float64,
    special map[string]map[string]float64,
    display map[string]map[string]bool,
) []WalletSpecialRatioRule {
    rules := make([]WalletSpecialRatioRule, 0)
    for userGroup, targets := range display {
        userRatios, ok := special[userGroup]
        if !ok {
            continue
        }
        for billingGroup, visible := range targets {
            specialRatio, specialOK := userRatios[billingGroup]
            baseRatio, baseOK := base[billingGroup]
            if !visible || !specialOK || !baseOK ||
                math.IsNaN(specialRatio) || math.IsInf(specialRatio, 0) ||
                math.IsNaN(baseRatio) || math.IsInf(baseRatio, 0) {
                continue
            }
            rules = append(rules, WalletSpecialRatioRule{
                UserGroup: userGroup, BillingGroup: billingGroup,
                SpecialRatio: specialRatio, BaseRatio: baseRatio,
            })
        }
    }
    sort.Slice(rules, func(i, j int) bool {
        if rules[i].UserGroup == rules[j].UserGroup {
            return rules[i].BillingGroup < rules[j].BillingGroup
        }
        return rules[i].UserGroup < rules[j].UserGroup
    })
    return rules
}
```

Add `math` and `sort` imports. Keep this helper in `service/group.go` because it is complex pricing projection logic with direct behavior tests.

- [ ] **Step 4: Add the controller and authenticated route**

Append to `controller/group.go`:

```go
func GetWalletSpecialRatioRules(c *gin.Context) {
    common.ApiSuccess(c, service.GetWalletSpecialRatioRules())
}
```

Add the missing `common` import. Inside the existing `selfRoute` block in `router/api-router.go`, register:

```go
selfRoute.GET("/wallet/special-ratios", controller.GetWalletSpecialRatioRules)
```

- [ ] **Step 5: Protect the authentication boundary with a router test**

Create `router/wallet_special_ratios_test.go`:

```go
package router

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestWalletSpecialRatiosRequireUserAuthentication(t *testing.T) {
    gin.SetMode(gin.TestMode)
    engine := gin.New()
    SetApiRouter(engine)

    recorder := httptest.NewRecorder()
    request := httptest.NewRequest(http.MethodGet, "/api/user/wallet/special-ratios", nil)
    engine.ServeHTTP(recorder, request)

    assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
```

- [ ] **Step 6: Run focused backend tests**

Run:

```powershell
go test ./service ./router -run 'TestBuildWalletSpecialRatioRules|TestWalletSpecialRatiosRequireUserAuthentication' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the authenticated projection**

```powershell
git add service/group.go service/group_wallet_ratio_test.go controller/group.go router/api-router.go router/wallet_special_ratios_test.go
git commit -m "feat(wallet): expose selected special ratios"
```

## Task 3: Add Wallet Visibility Controls to Group Pricing

**Files:**

- Create: `web/default/src/features/system-settings/models/group-ratio-wallet-display.ts`
- Create: `web/default/src/features/system-settings/models/group-ratio-wallet-display.test.ts`
- Modify: `web/default/src/features/system-settings/models/group-ratio-visual-editor.tsx`
- Modify: `web/default/src/features/system-settings/models/group-ratio-form.tsx`
- Modify: `web/default/src/features/system-settings/models/ratio-settings-card.tsx`
- Modify: `web/default/src/features/system-settings/billing/index.tsx`
- Modify: `web/default/src/features/system-settings/billing/section-registry.tsx`
- Modify: `web/default/src/features/system-settings/types.ts`

- [ ] **Step 1: Write failing pure helper tests**

Create `web/default/src/features/system-settings/models/group-ratio-wallet-display.test.ts`:

```ts
import { describe, expect, test } from 'bun:test'

import {
  buildWalletDisplayOptionUpdates,
  moveWalletDisplayRule,
  parseWalletDisplayMap,
  setWalletDisplayRule,
  validateWalletDisplayPairs,
} from './group-ratio-wallet-display'

describe('wallet special-ratio display settings', () => {
  test('selects, moves, and removes one pair without stale keys', () => {
    const selected = setWalletDisplayRule('{}', 'vip', 'premium', true)
    expect(parseWalletDisplayMap(selected)).toEqual({ vip: { premium: true } })

    const moved = moveWalletDisplayRule(
      selected,
      'vip',
      'premium',
      'vip',
      'pro'
    )
    expect(parseWalletDisplayMap(moved)).toEqual({ vip: { pro: true } })
    expect(parseWalletDisplayMap(setWalletDisplayRule(moved, 'vip', 'pro', false))).toEqual({})
  })

  test('rejects selected pairs absent from the ratio map', () => {
    expect(() =>
      validateWalletDisplayPairs(
        '{"vip":{"missing":true}}',
        '{"vip":{"premium":0.3}}'
      )
    ).toThrow('vip -> missing')
  })

  test('orders ratio update before wallet display update', () => {
    expect(
      buildWalletDisplayOptionUpdates(
        { GroupGroupRatio: '{"vip":{"premium":0.3}}', GroupGroupRatioWalletDisplay: '{"vip":{"premium":true}}' },
        { GroupGroupRatio: '{}', GroupGroupRatioWalletDisplay: '{}' }
      ).map((item) => item.key)
    ).toEqual([
      'GroupGroupRatio',
      'group_ratio_setting.group_group_ratio_wallet_display',
    ])
  })
})
```

- [ ] **Step 2: Run the helper test and verify it fails**

Run from `web/default`:

```powershell
bun test src/features/system-settings/models/group-ratio-wallet-display.test.ts
```

Expected: FAIL because `group-ratio-wallet-display.ts` does not exist.

- [ ] **Step 3: Implement the pure helper module**

Create `group-ratio-wallet-display.ts` with exported types and functions matching the tests:

```ts
export type WalletDisplayMap = Record<string, Record<string, true>>

export function parseWalletDisplayMap(value: string): WalletDisplayMap {
  const parsed: unknown = JSON.parse(value.trim() || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Wallet display rules must be a JSON object')
  }
  const result: WalletDisplayMap = {}
  for (const [userGroup, rawTargets] of Object.entries(parsed)) {
    if (!userGroup.trim() || !rawTargets || typeof rawTargets !== 'object' || Array.isArray(rawTargets)) {
      throw new Error('Wallet display groups must be non-empty JSON objects')
    }
    for (const [billingGroup, visible] of Object.entries(rawTargets)) {
      if (!billingGroup.trim() || typeof visible !== 'boolean') {
        throw new Error('Wallet display entries must use non-empty groups and boolean values')
      }
      if (!visible) continue
      result[userGroup] ??= {}
      result[userGroup][billingGroup] = true
    }
  }
  return result
}

export function serializeWalletDisplayMap(value: WalletDisplayMap): string {
  const sorted: WalletDisplayMap = {}
  for (const userGroup of Object.keys(value).sort()) {
    const targets = Object.keys(value[userGroup]).sort()
    if (targets.length === 0) continue
    sorted[userGroup] = {}
    for (const target of targets) sorted[userGroup][target] = true
  }
  return JSON.stringify(sorted, null, 2)
}

export function setWalletDisplayRule(value: string, userGroup: string, billingGroup: string, visible: boolean): string {
  const map = parseWalletDisplayMap(value)
  if (visible) {
    map[userGroup] ??= {}
    map[userGroup][billingGroup] = true
  } else if (map[userGroup]) {
    delete map[userGroup][billingGroup]
    if (Object.keys(map[userGroup]).length === 0) delete map[userGroup]
  }
  return serializeWalletDisplayMap(map)
}

export function isWalletDisplayRuleSelected(
  value: string,
  userGroup: string,
  billingGroup: string
): boolean {
  return Boolean(parseWalletDisplayMap(value)[userGroup]?.[billingGroup])
}
```

Implement `moveWalletDisplayRule` by reading whether the old pair is selected, removing it, and selecting the new pair only when needed. Implement `validateWalletDisplayPairs` by parsing the nested numeric ratio map and throwing an error containing `userGroup -> billingGroup` for a missing pair. Implement `buildWalletDisplayOptionUpdates` to normalize both values and return changed entries in the hard-coded order `GroupGroupRatio`, then the typed display option.

- [ ] **Step 4: Carry the new field through settings defaults and schema**

Add `GroupGroupRatioWalletDisplay: string` to the group settings types in:

- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/system-settings/models/group-ratio-form.tsx`
- `web/default/src/features/system-settings/models/ratio-settings-card.tsx`

Add default `'{}'` in `billing/index.tsx`, map the typed setting in `billing/section-registry.tsx`, and add the Zod field plus cross-field refinement:

```ts
GroupGroupRatioWalletDisplay: createJsonStringField(t),
```

```ts
.superRefine((values, context) => {
  try {
    validateWalletDisplayPairs(
      values.GroupGroupRatioWalletDisplay,
      values.GroupGroupRatio
    )
  } catch {
    context.addIssue({
      code: 'custom',
      path: ['GroupGroupRatioWalletDisplay'],
      message: t('Wallet display rules must reference existing special ratio rules'),
    })
  }
})
```

Replace the ad hoc group update loop with `buildWalletDisplayOptionUpdates(...)`, followed by the remaining unchanged group-setting updates in their current order. This guarantees the ratio write precedes the display write.

- [ ] **Step 5: Add the visual checkbox and JSON field**

Pass `groupGroupRatioWalletDisplay` from `GroupRatioForm` into `GroupRatioVisualEditor` and then `GroupOverrideRules`. Add a table column before Actions:

```tsx
{
  id: 'wallet-display',
  header: t('Wallet display'),
  cell: (override) => (
    <Checkbox
      checked={isWalletDisplayRuleSelected(
        groupGroupRatioWalletDisplay,
        userGroupData.userGroup,
        override.targetGroup
      )}
      onCheckedChange={(checked) =>
        onChange(
          'GroupGroupRatioWalletDisplay',
          setWalletDisplayRule(
            groupGroupRatioWalletDisplay,
            userGroupData.userGroup,
            override.targetGroup,
            checked === true
          )
        )
      }
      aria-label={t('Show this rule in the wallet')}
    />
  ),
},
```

Use `moveWalletDisplayRule` inside `handleOverrideSave` when the target group changes. Use `setWalletDisplayRule(..., false)` before deleting an override, and remove every selected target for a deleted source group. Add a JSON-mode `FormField` named `GroupGroupRatioWalletDisplay` with a textarea and the description `Selected special ratio rules shown in the wallet.`

- [ ] **Step 6: Run focused frontend settings tests and type checking**

Run from `web/default`:

```powershell
bun test src/features/system-settings/models/group-ratio-wallet-display.test.ts
bun run typecheck
```

Expected: both commands PASS.

- [ ] **Step 7: Commit the administrator controls**

```powershell
git add src/features/system-settings/models/group-ratio-wallet-display.ts src/features/system-settings/models/group-ratio-wallet-display.test.ts src/features/system-settings/models/group-ratio-visual-editor.tsx src/features/system-settings/models/group-ratio-form.tsx src/features/system-settings/models/ratio-settings-card.tsx src/features/system-settings/billing/index.tsx src/features/system-settings/billing/section-registry.tsx src/features/system-settings/types.ts
git commit -m "feat(settings): select wallet-visible special ratios"
```

## Task 4: Render the Independent Wallet Card and Responsive Grid

**Files:**

- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/api.ts`
- Create: `web/default/src/features/wallet/lib/special-ratios.ts`
- Create: `web/default/src/features/wallet/lib/special-ratios.test.ts`
- Create: `web/default/src/features/wallet/components/special-ratio-rules-card.tsx`
- Modify: `web/default/src/features/wallet/index.tsx`

- [ ] **Step 1: Write failing wallet state and layout tests**

Create `web/default/src/features/wallet/lib/special-ratios.test.ts`:

```ts
import { describe, expect, test } from 'bun:test'

import {
  getSpecialRatioCardState,
  getWalletPrimaryGridClass,
} from './special-ratios'

describe('wallet special ratio card', () => {
  test('keeps loading and errors visible but hides a successful empty result', () => {
    expect(getSpecialRatioCardState({ isPending: true, isError: false, count: 0 })).toEqual({ display: 'loading', available: true })
    expect(getSpecialRatioCardState({ isPending: false, isError: true, count: 0 })).toEqual({ display: 'error', available: true })
    expect(getSpecialRatioCardState({ isPending: false, isError: false, count: 0 })).toEqual({ display: 'hidden', available: false })
    expect(getSpecialRatioCardState({ isPending: false, isError: false, count: 2 })).toEqual({ display: 'rules', available: true })
  })

  test('selects one, two, and three-card grid classes', () => {
    expect(getWalletPrimaryGridClass(false, false)).toContain('grid-cols-1')
    expect(getWalletPrimaryGridClass(true, false)).toContain('xl:grid-cols-2')
    expect(getWalletPrimaryGridClass(false, true)).toContain('xl:grid-cols-2')
    expect(getWalletPrimaryGridClass(true, true)).toContain('2xl:grid-cols-3')
  })
})
```

- [ ] **Step 2: Run the wallet test and verify it fails**

Run from `web/default`:

```powershell
bun test src/features/wallet/lib/special-ratios.test.ts
```

Expected: FAIL because `special-ratios.ts` does not exist.

- [ ] **Step 3: Add wallet API types and pure view helpers**

In `wallet/types.ts`, add:

```ts
export type WalletSpecialRatioRule = {
  user_group: string
  billing_group: string
  special_ratio: number
  base_ratio: number
}

export type WalletSpecialRatioResponse = ApiResponse<WalletSpecialRatioRule[]>
```

In `wallet/api.ts`, add:

```ts
export async function getWalletSpecialRatioRules(): Promise<WalletSpecialRatioRule[]> {
  const response = await api.get<WalletSpecialRatioResponse>(
    '/api/user/wallet/special-ratios'
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to load special ratios')
  }
  return response.data.data ?? []
}
```

Create `wallet/lib/special-ratios.ts`:

```ts
export type SpecialRatioCardDisplay = 'loading' | 'error' | 'hidden' | 'rules'

export function getSpecialRatioCardState(input: {
  isPending: boolean
  isError: boolean
  count: number
}): { display: SpecialRatioCardDisplay; available: boolean } {
  if (input.isPending) return { display: 'loading', available: true }
  if (input.isError) return { display: 'error', available: true }
  if (input.count === 0) return { display: 'hidden', available: false }
  return { display: 'rules', available: true }
}

export function getWalletPrimaryGridClass(
  subscriptionAvailable: boolean,
  specialRatiosAvailable: boolean
): string {
  if (subscriptionAvailable && specialRatiosAvailable) {
    return 'grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3 2xl:items-start'
  }
  if (subscriptionAvailable || specialRatiosAvailable) {
    return 'grid grid-cols-1 gap-4 xl:grid-cols-2 xl:items-start'
  }
  return 'grid grid-cols-1 gap-4'
}
```

- [ ] **Step 4: Build the card component**

Create `wallet/components/special-ratio-rules-card.tsx`. Use `useQuery` with query key `['wallet-special-ratios']`, call `getSpecialRatioCardState`, and notify the parent whenever `available` changes:

```tsx
const query = useQuery({
  queryKey: ['wallet-special-ratios'],
  queryFn: getWalletSpecialRatioRules,
})
const state = getSpecialRatioCardState({
  isPending: query.isPending,
  isError: query.isError,
  count: query.data?.length ?? 0,
})

useEffect(() => {
  props.onAvailabilityChange?.(state.available)
}, [props.onAvailabilityChange, state.available])

if (state.display === 'hidden') return null
```

Render one `TitledCard` titled `Special ratios`. For loading, render three fixed-height `Skeleton` rows. For errors, render `Failed to load special ratios` and an icon Retry button calling `query.refetch()`. For rules, render separator rows with a truncated `user_group -> billing_group` label, prominent `${special_ratio}x`, and supporting `Base ratio {{ratio}}x`. Wrap long group labels in the existing `Tooltip` primitives.

- [ ] **Step 5: Mount the card and replace the wallet grid expression**

In `wallet/index.tsx`, initialize:

```ts
const [showSpecialRatioPanel, setShowSpecialRatioPanel] = useState(true)
```

Replace the current conditional grid class with:

```tsx
<div className={getWalletPrimaryGridClass(
  showSubscriptionPanel,
  showSpecialRatioPanel
)}>
```

Keep Recharge first and Subscription second, then mount:

```tsx
<SpecialRatioRulesCard
  onAvailabilityChange={setShowSpecialRatioPanel}
/>
```

This preserves the required order at every breakpoint and still shows the new card when subscriptions are unavailable.

- [ ] **Step 6: Run focused wallet tests and type checking**

Run from `web/default`:

```powershell
bun test src/features/wallet/lib/special-ratios.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit the wallet experience**

```powershell
git add src/features/wallet/types.ts src/features/wallet/api.ts src/features/wallet/lib/special-ratios.ts src/features/wallet/lib/special-ratios.test.ts src/features/wallet/components/special-ratio-rules-card.tsx src/features/wallet/index.tsx
git commit -m "feat(wallet): show selected special ratio rules"
```

## Task 5: Complete i18n and End-to-End Verification

**Files:**

- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/zh-TW.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify if generated: `web/default/src/i18n/locales/_reports/_sync-report.json`

- [ ] **Step 1: Run i18n synchronization to expose every new literal key**

From `web/default`, run:

```powershell
bun run i18n:sync
```

Expected: the locale report lists the new English keys, including `Wallet display`, `Show this rule in the wallet`, `Selected special ratio rules shown in the wallet.`, `Special ratios`, `Base ratio {{ratio}}x`, and `Failed to load special ratios`.

- [ ] **Step 2: Fill every locale and verify zero gaps**

Translate every new key in `en`, `zh`, `zh-TW`, `fr`, `ja`, `ru`, and `vi`. Run:

```powershell
bun run i18n:sync
```

Expected in `_reports/_sync-report.json` for every locale:

```json
{
  "missingCount": 0,
  "extrasCount": 0,
  "untranslatedCount": 0
}
```

- [ ] **Step 3: Run focused and broad backend verification**

From the repository root, run:

```powershell
go test ./setting/ratio_setting ./service ./router -count=1
go test ./... -count=1
```

Expected: both commands PASS.

- [ ] **Step 4: Run focused and broad frontend verification**

From `web/default`, run:

```powershell
bun test src/features/system-settings/models/group-ratio-wallet-display.test.ts src/features/wallet/lib/special-ratios.test.ts
bun test
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/system-settings/models/group-ratio-wallet-display.ts src/features/system-settings/models/group-ratio-wallet-display.test.ts src/features/system-settings/models/group-ratio-visual-editor.tsx src/features/system-settings/models/group-ratio-form.tsx src/features/system-settings/models/ratio-settings-card.tsx src/features/system-settings/billing/index.tsx src/features/system-settings/billing/section-registry.tsx src/features/system-settings/types.ts src/features/wallet/types.ts src/features/wallet/api.ts src/features/wallet/lib/special-ratios.ts src/features/wallet/lib/special-ratios.test.ts src/features/wallet/components/special-ratio-rules-card.tsx src/features/wallet/index.tsx
bun run build
```

Expected: all commands PASS with no lint errors or TypeScript errors.

- [ ] **Step 5: Run browser verification**

Start the existing local backend and the frontend dev server, then verify with Playwright at 1600x1000, 1280x800, 768x1024, and 390x844:

1. In Group Pricing, add a special ratio, leave Wallet display off, save, and confirm the wallet card is absent.
2. Enable Wallet display, save, reload settings, and confirm the checkbox persists.
3. Open Wallet and confirm the independent card shows exact source group, billing group, special ratio, and base ratio.
4. Confirm the 1600px layout uses three columns, the normal desktop/tablet layouts stay contained, and mobile stacks Recharge, Subscription, then Special ratios.
5. Delete the ratio rule, save, and confirm the wallet card disappears without affecting recharge or subscription controls.
6. Check the browser console and network panel for no React errors and a successful authenticated `/api/user/wallet/special-ratios` request.

Expected: no overlap, clipping, blank card, console error, failed asset, or unrelated workflow regression.

- [ ] **Step 6: Commit translations and verification fixes**

```powershell
# Run this commit command from the repository root after the frontend commands.
git add web/default/src/i18n/locales web/default/src/features/system-settings web/default/src/features/wallet setting/ratio_setting service controller router model
git commit -m "chore(wallet): complete special ratio verification"
```

## Final Completion Check

- [ ] Confirm `git status --short` is empty.
- [ ] Confirm the branch contains the design commit, implementation-plan commit, and the five implementation commits.
- [ ] Confirm no protected project identity, unrelated metadata, database data, or existing user changes were modified.
- [ ] Record the exact Go, Bun, typecheck, lint, i18n, build, and browser verification results before claiming completion.
