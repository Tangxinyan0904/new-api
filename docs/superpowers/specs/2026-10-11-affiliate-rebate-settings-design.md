# Configurable Invitation Rebate Settings Design

Date: 2026-10-11

## Goal

Replace the hard-coded invitation rebate rules (5% rate, $1 minimum, one
request per day, mandatory approval, forfeiture on rejection) with an
administrator-managed **Invitation Rebate** section under
System Settings -> Billing.

## Confirmed Decisions

- Changing the rebate rate or switch only affects top-ups completed after the
  change. Rebates already earned keep the rate that applied at completion.
- The section contains: rebate switch, rebate rate, minimum transfer amount,
  daily request limit, require administrator approval, forfeit on rejection,
  and the invitation rewards (inviter / invitee), which move out of
  Quota Settings.
- Turning the rebate off freezes earned recharge rebate: it is hidden from the
  wallet and cannot be transferred until the rebate is turned on again.
  Invitation rewards stay transferable.

## Settings

Registered as `affiliate_setting.*` in the global config manager:

| Key | Default | Meaning |
| --- | --- | --- |
| `rebate_enabled` | `true` | New top-ups earn rebate; earned rebate visible and transferable |
| `rebate_rate_percent` | `5` | 0-100, at most two decimals |
| `min_transfer_quota` | `500000` | Smallest transfer; `0` = any positive amount |
| `daily_request_limit` | `1` | Requests per user per local day; `0` = unlimited |
| `require_approval` | `true` | `false` approves and credits on submission |
| `forfeit_on_reject` | `true` | `false` returns rejected amounts to the pool |

Defaults reproduce the previous behavior. The whole form is saved through
`PUT /api/option/affiliate` (root only). The generic option endpoint rejects
`affiliate_setting.*` keys so the rate history below cannot be bypassed.

## Rate History

`affiliate_rebate_rate_changes` stores `(effective_at, enabled,
rate_basis_points, changed_by)`. A row is appended in the same transaction as
the option write whenever the switch or rate differs from the latest row.

A top-up earns the rate in effect at `complete_time` (falling back to
`create_time`). Before the first row every top-up uses the legacy 5%, so
existing balances are unchanged after upgrade. Credited quota is grouped by
rate before rounding, which keeps the legacy aggregate rounding exactly when
no history exists. No payment completion path is modified.

## Transfer Flow

- Creation checks, in order: pending request, daily limit, nothing available,
  minimum. Daily limits use the deployment's local day instead of UTC.
- Without approval, the request is approved in the creating transaction and
  a user-visible `affiliate.transfer.auto_approved` log is written.
- Rejection forfeits (deducts invitation reward and marks rebate consumed)
  only when `forfeit_on_reject` is on at rejection time.
- The legacy `POST /api/user/aff_transfer` now runs the same request flow. It
  previously moved `aff_quota` to the balance directly, bypassing approval.
- The inviter registration reward uses an atomic column update instead of a
  read-modify-save of the whole user row (ported from upstream).

## Testing

Model tests cover per-period rates, legacy rounding, freeze and unfreeze,
auto-approval crediting, configurable daily limit and minimum, rejection
without forfeiture, history recording only on change, and validation.
Controller tests cover the dedicated endpoint, incomplete and invalid forms,
the compliance requirement for invitation rewards, and the generic endpoint
rejecting `affiliate_setting.*` keys.
