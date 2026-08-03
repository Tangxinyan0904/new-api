# Wallet Special Ratio Display Design

Date: 2026-07-29

## Scope

This specification covers only the first item in the current request: allowing
administrators to select special group-ratio rules for display in a dedicated
card beside the subscription card on the wallet page. Log presentation, log
group filtering, and the API-key announcement are separate follow-up designs.

## Background

Group pricing currently stores special ratio overrides in `GroupGroupRatio` as
a nested map:

```json
{
  "vip": {
    "premium": 0.3
  }
}
```

The outer key is the user's group, the inner key is the billing/token group,
and the number is the effective ratio. The setting is consumed directly by
billing, so changing its value shape would create compatibility and accounting
risk. The wallet currently renders a recharge card and, when available, a
subscription card. It does not expose selected group-pricing offers.

## Goals

1. Add a per-rule administrator choice for wallet visibility without changing
   billing behavior or the `GroupGroupRatio` format.
2. Show every selected rule to every authenticated wallet user, regardless of
   that user's current group.
3. Render selected rules in a separate wallet card beside the subscription
   card on wide screens and in the normal vertical flow on smaller screens.
4. Generate display text from authoritative pricing data rather than requiring
   administrators to maintain duplicate custom copy.
5. Hide the card completely when no valid rules are selected.

## Confirmed Decisions

- Visibility metadata is stored separately from the billing ratio map.
- All authenticated wallet users see the same selected-rule list.
- The wallet uses a separate titled card, not content nested inside the
  subscription card and not a banner.
- Each row automatically shows the source user group, billing group, special
  ratio, and billing group's base ratio.
- New special ratio rules are not displayed by default.
- Removing a special ratio rule also removes its wallet-display selection.
- Invalid or stale display references are never returned to wallet users.

## Configuration Model

Add the typed system option:

`group_ratio_setting.group_group_ratio_wallet_display`

The group-pricing frontend field is named `GroupGroupRatioWalletDisplay`. Its
serialized shape mirrors the ratio map but stores booleans:

```json
{
  "vip": {
    "premium": true
  }
}
```

Only `true` entries are meaningful. The parser rejects malformed JSON,
non-object levels, empty group names, and non-boolean values. Empty inner maps
are removed during normalization. Output is normalized into stable sorted keys
so form dirty-state and tests are deterministic.

The backend setting package exposes copied snapshots and a lookup method; it
does not expose the mutable internal map. JSON marshal and unmarshal operations
use the project `common` wrappers.

## Cross-Field Validation and Save Order

Every selected `(user group, billing group)` pair must exist in the submitted
`GroupGroupRatio` map. The visual editor prevents selecting a missing rule and
removes a selection when its ratio rule is deleted. The JSON editor validates
the same invariant before submission.

The existing group-pricing save flow updates changed options sequentially. It
must save `GroupGroupRatio` before
`group_ratio_setting.group_group_ratio_wallet_display`. This order supports
creating a rule and selecting it in one save. If the ratio update succeeds but
the display update fails, the rule remains valid billing configuration and is
simply not advertised, which is the conservative failure mode.

When a ratio rule is removed, the frontend submits the pruned visibility map
after the ratio map. During the short interval between those writes, the user
endpoint independently verifies every selection against the current ratio map
and omits the stale pair. A failed visibility cleanup therefore cannot expose a
nonexistent rule.

Direct updates to the display option receive the same backend cross-field
validation against the current `GroupGroupRatio` snapshot.

## Administrator UI

The visual editor's Special ratio rules table adds a `Wallet display` checkbox
column. The checkbox is keyed by the source user group and target billing group
pair. It is unchecked for a new rule, preserved while editing the rule ratio,
and removed when either the individual override or its source-group section is
deleted.

Renaming a target group through the existing edit dialog moves the visibility
selection to the new pair only when the original pair was selected. This keeps
an administrator's intent while avoiding an orphaned key.

The group-pricing form includes the visibility map in its schema, normalized
defaults, dirty tracking, option-key mapping, save operation, and reset logic.
JSON mode adds a labeled textarea and validation message for the same field so
switching editor modes cannot hide or discard configuration.

All new labels, descriptions, validation messages, and wallet copy use literal
`t('English source')` keys and are translated for all supported locales.

## Authenticated Wallet API

Add an authenticated read-only endpoint under the existing user API routes:

`GET /api/user/wallet/special-ratios`

The response data is a JSON array:

```json
[
  {
    "user_group": "vip",
    "billing_group": "premium",
    "special_ratio": 0.3,
    "base_ratio": 0.5
  }
]
```

The controller/service builds the response from a single consistent snapshot
of:

- the wallet-display selection map;
- the special ratio map;
- the base group ratio map.

For every selected pair, it verifies that the special rule still exists and
that the billing group has a base ratio. Missing or non-finite ratios are
skipped and logged as configuration errors; they are not sent to clients. The
result is sorted by `user_group`, then `billing_group`, using exact string
ordering so every authenticated user receives the same stable list.

The endpoint returns an empty array when nothing valid is selected. It does not
return unselected special rules or the raw configuration maps. Authentication
prevents adding this administrative pricing information to the public status
payload.

## Wallet Card

Add a focused `SpecialRatioRulesCard` component to the wallet feature. It loads
the authenticated endpoint independently from recharge and subscription data.
An endpoint failure cannot block either existing workflow.

Each displayed row contains:

- source user group and billing group as `vip -> premium`;
- the special value as `0.3x`;
- supporting text for the base value, such as `Base ratio 0.5x`.

The component uses one titled card with unframed rows separated by borders; it
does not nest cards. Long group names truncate with a tooltip, while ratios
remain visible. The list may grow naturally with the selected rules rather than
hiding valid entries behind an arbitrary count limit.

While loading, the component reserves a stable card-sized skeleton. On an API
error it shows a compact error state with a retry command inside the same card.
After a successful empty response, it renders nothing and reports itself as
unavailable to the wallet layout.

## Responsive Wallet Layout

The wallet parent tracks the availability of both the subscription card and
the special-ratio card. Layout behavior is:

- recharge + subscription + special ratios: three columns on sufficiently wide
  desktop screens, with the special-ratio card immediately after the
  subscription card;
- any two available cards: two balanced desktop columns;
- recharge only: the existing single-column layout;
- tablet and mobile: one vertical column in the same logical order.

The loading skeleton counts as available while the request is pending, so a
non-empty result replaces it without resizing the grid. A successful empty
result intentionally collapses the unused column once. Fixed minimum track
sizes and `minmax(0, 1fr)` keep translated text and long group names from
widening the page.

The special-ratio card remains visible when selected rules exist even if there
are no public subscription plans.

## Error Handling

- Invalid administrator JSON is rejected before any option write.
- A display pair that does not exist in the current special ratio map is
  rejected on direct configuration writes.
- Stale pairs caused by a partial multi-option save are ignored by the wallet
  endpoint until the next successful cleanup save.
- Non-finite or missing ratio values are treated as server configuration
  errors and omitted from the response.
- Wallet fetch errors are isolated to the new card and offer a retry action.
- No error path changes billing ratios, subscription availability, recharge,
  or payment behavior.

## Testing

Backend tests protect:

- display-map JSON validation and deterministic normalization;
- copied snapshot behavior and concurrent reads;
- cross-field validation against `GroupGroupRatio`;
- selected valid rules returned with exact special and base ratios;
- unselected, deleted, missing-base, and non-finite entries omitted;
- deterministic response ordering;
- authentication on the wallet endpoint;
- an empty valid configuration returning an empty array.

Frontend tests protect:

- new rules defaulting to wallet display off;
- checkbox selection surviving save and reload;
- edit/rename/delete behavior keeping the selection map consistent;
- JSON and visual mode parity;
- ratio settings saving the billing map before the display map;
- automatic row content and exact ratio formatting;
- loading, empty, error, retry, and successful card states;
- one-, two-, and three-card responsive layout selection;
- long translated group labels remaining contained;
- complete i18n synchronization for every supported locale.

Verification includes focused Go and frontend tests, affected package tests,
`go test ./...`, frontend type checking, lint on changed frontend files, i18n
checks, the production frontend build, and browser screenshots at wide desktop,
standard desktop, tablet, and narrow mobile widths.

## Non-Goals

- Changing billing calculations or the `GroupGroupRatio` value format.
- Filtering advertised rules by the current user's group.
- Allowing custom per-rule marketing copy.
- Publishing the selected rules in the unauthenticated status response.
- Implementing the separate log or API-key announcement requests in this
  subproject.

## Acceptance Criteria

1. An administrator can select any existing special ratio rule for wallet
   display and save that choice without changing its billing ratio.
2. Every authenticated wallet user receives the same selected valid rules and
   no unselected rules.
3. The wallet shows a separate card beside the subscription card on wide
   screens and a contained vertical layout on smaller screens.
4. Each row automatically displays source group, billing group, special ratio,
   and base ratio.
5. The card is hidden after a successful empty response and remains independent
   from recharge and subscription failures.
6. Rule edits and deletions cannot leave a user-visible stale selection.
7. Backend tests, frontend tests, type checks, i18n checks, builds, and browser
   verification pass without changing billing behavior.
