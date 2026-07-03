# API Keys Daily Usage and Wallet Rebate Approval Design

Date: 2026-07-03

## Scope

This design covers two requested changes:

1. API key management adds a visible "Today Usage" column beside the existing quota column, while keeping the expiration column available but hidden by default.
2. Wallet referral rewards gain a separate recharge rebate area based on invited users' successful credited recharge quota. Transfers from pending referral/rebate balance to usable balance become an admin-approved workflow.

## Decisions Confirmed

- Recharge rebate rate is 5%.
- The 5% rebate is calculated from credited recharge quota, not cash paid. In existing records this means the quota credited by the successful top-up flow, normalized to internal quota units.
- Invitation registration rewards and recharge rebates must be shown separately, even though they can both contribute to a user's pending withdrawable amount.
- Invited users' identities must be privacy protected. The UI may show masked nicknames/usernames, but must not show exact per-user recharge amounts.
- Users can submit at most one transfer request per day. After submission, the transfer button should show an approving/pending state.
- Admins need a new rebate approval page with approve and reject actions.

## Task 1: API Key Today Usage

### Backend

Extend token listing responses for:

- `GET /api/token/`
- `GET /api/token/search`

Each token item should include a `today_quota` field.

`today_quota` is calculated by summing `logs.quota` where:

- `logs.token_id = tokens.id`
- `logs.type = LogTypeConsume`
- `logs.created_at >= local start of today`
- `logs.created_at <= now`

The aggregation should be batched for all token IDs on the current page to avoid one query per row. If there are no matching logs, the field returns `0`.

### Frontend

Update `web/default/src/features/keys/components/api-keys-columns.tsx`:

- Add `today_quota` immediately after `quota`.
- Render it with the same quota formatting helper used elsewhere.
- Keep it visible by default on desktop.
- Mobile can show it in the mobile card under quota if space allows; otherwise desktop-only is acceptable for the first pass.

Update `web/default/src/features/keys/components/api-keys-table.tsx`:

- Set `expired_time: false` in `initialColumnVisibility`.
- Keep `expired_time` in the column settings so users can re-enable it.

## Task 2: Wallet Recharge Rebate and Approval

## Data Model

Add a new approval model, for example `AffiliateTransferRequest`, with fields:

- `id`
- `user_id`
- `username` or query-time joined username
- `invite_reward_quota`
- `recharge_rebate_quota`
- `total_quota`
- `status`: `pending`, `approved`, `rejected`
- `created_at`
- `reviewed_at`
- `reviewed_by`
- `review_note` or `reject_reason`

Keep this separate from `users.aff_quota` so registration invite rewards and recharge rebates remain auditable separately.

Add helper logic to calculate a user's recharge rebate summary:

1. Find invited users where `users.inviter_id = current_user.id`.
2. Sum successful top-ups for those invited users.
3. Convert each successful top-up to credited quota using the same semantics as the recharge completion flow:
   - For providers where `amount` represents display units, convert to internal quota as the existing completion path does.
   - For providers where `amount` already stores internal credited quota, use that credited quota directly.
4. Rebate quota is `floor(total_invited_credited_quota * 0.05)`.
5. Pending recharge rebate should subtract amounts already requested/approved as recharge rebate quota, so it is not withdrawable twice.

The API should return both raw totals and formatted frontend-ready numeric values, but the frontend will still format quota using existing helpers.

## User APIs

Add endpoints:

- `GET /api/user/affiliate/rebate-summary`
  - Returns masked invited user names, invited user count, total invited credited recharge quota, registration invite pending quota, recharge rebate pending quota, total pending quota, and any current pending request.
- `POST /api/user/affiliate/transfer-request`
  - Creates a pending transfer request for all currently withdrawable invite reward + rebate quota, or for a requested amount if the UI later supports partial transfer.
  - Enforces one request per user per local day.
  - Rejects when there is already a pending request.

Existing `/api/user/aff_transfer` should not directly transfer balance anymore for the new frontend flow. It can either be kept for compatibility or internally routed to the request creation logic.

## Admin APIs

Add endpoints under admin auth:

- `GET /api/user/affiliate/transfer-requests`
  - Paginated list of transfer requests.
  - Optional status filter.
- `POST /api/user/affiliate/transfer-requests/:id/approve`
  - Approves a pending request in a transaction.
  - Revalidates available pending invite reward and rebate quota before applying.
  - Deducts registration invite reward from `users.aff_quota` as needed.
  - Marks rebate quota as paid through the approval record.
  - Adds `total_quota` to `users.quota`.
- `POST /api/user/affiliate/transfer-requests/:id/reject`
  - Marks the request rejected with optional reason.
  - Does not mutate user quota.

Approval and rejection should record operation logs for auditability.

## Wallet UI

Keep the existing referral card for referral link and summary, but extend the area under the recommended plans with a new card that shows:

- Masked nicknames/usernames of invited users.
- Invited user count.
- Total invited credited recharge quota.
- Invitation reward pending amount.
- Recharge rebate pending amount.
- Total pending withdrawable balance.
- Transfer request status.

Privacy rule:

- Show masked names only, for example `Mi***`, `user***123`, or `张*`.
- Do not show per-invited-user recharge amounts.
- Show only aggregate invited recharge quota.

Transfer button states:

- No pending rewards: disabled.
- Pending request exists: disabled with `审批中`.
- Request already submitted today: disabled with a daily-limit message if no pending request remains.
- Available and no daily block: enabled as `提交划转申请`.

## Admin UI

Add sidebar item under Admin:

- Title: `返利审批`
- Route: `/rebate-approvals`
- Icon: a finance/approval icon from `lucide-react`, such as `BadgeCheck`, `CircleDollarSign`, or `ReceiptText`.

Add frontend route:

- `web/default/src/routes/_authenticated/rebate-approvals/index.tsx`

Add feature module:

- `web/default/src/features/rebate-approvals/`

The page uses the existing `SectionPageLayout` and `DataTablePage` patterns. Columns:

- Applicant
- Invitation reward quota
- Recharge rebate quota
- Total quota
- Status
- Created time
- Reviewed time/reviewer
- Actions

Actions:

- Pending rows show `通过` and `拒绝`.
- Non-pending rows show status only.

## Migration and Compatibility

- Add the approval table through GORM auto-migration.
- Keep compatibility with SQLite, MySQL, and PostgreSQL.
- Avoid database-specific SQL in migrations.
- Daily request limiting should use timestamp ranges rather than database-specific date functions.
- Rebate aggregation should avoid exposing per-invitee payment amounts through API responses.

## Testing and Verification

Backend checks:

- Token list includes correct `today_quota` aggregation.
- Expired column hidden by default in frontend table state.
- Rebate summary masks invited users and returns aggregate recharge totals only.
- Rebate calculation uses credited quota amount and 5% rate.
- One transfer request per user per day is enforced.
- Pending request blocks duplicate submission.
- Approval transfers quota exactly once.
- Rejection does not transfer quota.

Frontend checks:

- API key table shows Today Usage next to Quota.
- Expires column is hidden by default but available in column settings.
- Wallet rebate card displays invitation reward and recharge rebate separately.
- Pending transfer request changes button to approval-in-progress state.
- Admin rebate approval page can approve/reject pending requests.

## Implementation Notes

Prefer a small backend service layer for rebate summary and request approval so all routes share the same calculation and transaction rules. Do not duplicate rebate math in controllers.
