# Affiliate Approval Log and Registration IP Abuse Protection Design

Date: 2026-07-29

## Background

Affiliate transfer approval currently credits the requested quota and writes an
administrator-owned audit log. The credited user has no corresponding entry in
their own usage-log history, so they cannot see why their balance increased.

Self-service registration currently creates users through password, built-in
OAuth, custom OAuth, Telegram, and WeChat paths. The user record does not retain
the registration IP, and generic route throttling cannot enforce a durable
three-account rule or safely restore only accounts disabled by that rule.

## Goals

1. Write a user-owned management log after a successful affiliate transfer
   approval, showing the amount approved by an administrator.
2. Allow at most three self-service registrations per exact IP in one counting
   cycle.
3. Create the fourth account, then atomically disable all four accounts and
   block further registrations from that IP.
4. Cover every self-service registration path while excluding administrator
   creation, initial setup, and historical accounts.
5. Let administrators search blocked IPs and associated accounts, unblock an
   IP, and manage exact-IP allowlist entries under System Settings -> Security.
6. Preserve administrator status decisions and support SQLite, MySQL, and
   PostgreSQL.

## Confirmed Decisions

- The limit is three self-service accounts per exact IP.
- The fourth account is persisted, then it and the first three accounts in the
  current cycle are disabled.
- A blocked IP cannot register another account until an administrator unblocks
  it.
- Password, GitHub, Discord, OIDC, Telegram, WeChat, custom OAuth, and other
  self-service paths using the shared OAuth creation flow are covered.
- Administrator-created users and the initial root account are excluded.
- Historical users are not backfilled and do not count toward the limit.
- Unblocking restores only accounts still eligible for automatic restoration,
  clears the count, and starts a new cycle.
- A later administrator disable removes that account's automatic-restoration
  eligibility, so IP unblocking cannot override the administrator.
- Adding an IP to the allowlist also unblocks it, restores eligible accounts,
  and clears the count.
- Removing an IP from the allowlist starts a new empty counting cycle.
- Allowlist entries accept exact IPv4 or IPv6 addresses only. CIDR ranges are
  not accepted.
- The management entry is System Settings -> Security -> Registration Abuse
  Protection.

## Affiliate Approval User Log

The successful approval flow keeps its current administrator audit log and adds
a second log owned by the credited user. The second record uses LogTypeManage
and a stable structured action, with parameters containing the request ID and
approved quota. Its fallback content is Administrator approved {{amount}}
balance.

The default frontend renders the localized equivalent, including the requested
Chinese wording 管理员批准 {{amount}} 余额, using the same quota-to-balance
formatting as other usage-log amounts. Administrator identity is nested under
other.admin_info. Existing log sanitization keeps that metadata administrator-
only; the ordinary user sees the localized content and amount.

The user log is written only after the approval transaction succeeds. Approval
failure, rollback, duplicate approval, or a request that is no longer pending
produces no user log. As with existing audit logging, a log-storage failure is
reported but does not reverse an already committed balance credit.

## IP Normalization

Each self-service entry point passes Gin's ClientIP result into a shared
registration service. This preserves the project's trusted-proxy configuration
and does not add trust for arbitrary forwarding headers.

The service uses Go's net/netip parser, removes an IPv6 zone if one is present,
unmaps IPv4-in-IPv6 addresses, and stores the canonical Addr.String result.
Empty or invalid IP data fails self-service registration conservatively and is
never stored as a shared empty key. Administrator and setup creation do not use
this service and do not require a registration IP.

## Persistence Model

Add registration_ip_states, one row per canonical IP, containing:

- a primary key;
- canonical IP with a unique index;
- a positive current cycle number;
- the current-cycle registration count;
- blocked-at Unix timestamp, where zero means unblocked;
- allowlisted state;
- created-at and updated-at Unix timestamps.

Add registration_ip_accounts, one row per self-service-created user, containing:

- a primary key;
- a unique indexed user ID;
- indexed canonical registration IP;
- indexed registration cycle;
- auto-disabled-at Unix timestamp;
- restore-eligible state;
- created-at and updated-at Unix timestamps.

The account table is the authoritative association used by the administrator
panel and restoration flow. It intentionally has no database-enforced cascade,
so a soft-deleted user remains auditable. Administrator queries join users with
unscoped semantics and label deleted accounts; deleted accounts are never
restored.

Business defaults are set in Go rather than boolean default tags. GORM creates
portable columns and indexes without dialect-specific JSON, date types, partial
indexes, or ALTER COLUMN statements.

## Atomic Registration Flow

Self-service user creation moves behind a shared transaction-aware method that
receives the canonical IP and provider-specific transaction work, such as an
OAuth binding. Its flow is:

1. Normalize the client IP.
2. Start a main-database transaction.
3. Find or create the IP state and load it through lockForUpdate.
4. If it is allowlisted, create the user and association without counting or
   enforcement.
5. If it is blocked, reject before creating a user.
6. Otherwise create the user and provider binding, create the IP association,
   and increment the current-cycle count.
7. When the count becomes four, update every still-enabled user associated with
   that IP and cycle to disabled. Mark only those users restore-eligible and mark
   the IP blocked in the same transaction.
8. Commit, invalidate every affected user-status cache, and run the existing
   post-registration behavior.

The fourth registration returns a stable result explaining that the account was
created but disabled because the IP limit was exceeded. Password registration
does not establish a login session, and OAuth, Telegram, and WeChat do not
establish a session for the disabled account.

Existing account creation side effects remain unchanged: the fourth account is
not deleted, and this feature does not claw back balances, tokens, or affiliate
rewards. Those are separate accounting policies. Disabled users and their API
tokens remain blocked by the existing user-status checks.

The unique IP constraint handles state-creation races, with a bounded retry of
the whole transaction when necessary. Existing state rows are locked before any
count or status change. SQLite relies on its serialized writer behavior; MySQL
and PostgreSQL use the shared row-lock helper. Concurrent registrations cannot
leave more than three current-cycle accounts enabled or disable only a subset of
the fourth-account group.

## Manual User Status Interaction

When an administrator explicitly disables a user, the same transaction clears
restore eligibility on that user's registration-IP association. This applies
even if the account is already disabled by the automated rule, so a deliberate
administrator action cannot later be undone by IP unblocking.

Explicitly enabling an account clears its automatic-restoration marker but does
not unblock the IP or change the IP counter. The dedicated unblock action is
still required before that IP may register again.

## Unblock and Allowlist Transactions

Unblocking locks the IP state and finds current-cycle associations marked
restore-eligible. It re-enables only users that are still disabled, not deleted,
and still eligible. It then clears the restoration markers, increments the
cycle, resets the count to zero, clears the blocked timestamp, and commits.
Affected user-status caches are invalidated after commit.

Adding an exact IP to the allowlist performs the same unblock-and-restore work
and then marks the IP allowlisted. Removing an allowlist entry increments the
cycle, resets the count and block state, and removes allowlisted status. The
first later registration therefore counts as one.

All three operations are idempotent where practical and write administrator
audit logs containing the canonical IP, affected-account count, affected user
IDs, and resulting state. APIs never return credentials, access tokens, OAuth
identifiers, or API-token secrets.

## Administrator API

Add administrator-authorized endpoints under the existing admin API group:

- GET /api/registration-ip-abuse/blocked
- POST /api/registration-ip-abuse/:ip/unblock
- GET /api/registration-ip-abuse/allowlist
- POST /api/registration-ip-abuse/allowlist
- DELETE /api/registration-ip-abuse/allowlist/:ip

The blocked list uses the normal page and page-size parameters. Its optional
search matches an exact canonical IP or associated users by numeric ID,
username, or display name, following existing user-search behavior. Results are
grouped by IP and ordered by newest block first. Each item includes block time,
current-cycle count, associated-account count, and accounts with numeric ID,
username, display name, registration time, deleted/current status, and
restoration eligibility.

The allowlist mutation canonicalizes one exact IP. A duplicate add succeeds
idempotently. CIDR, hostnames, ports, empty values, and malformed addresses
return a stable validation error.

## Security Settings UI

Register a new Security section titled Registration Abuse Protection. It
contains:

- a compact fixed-rule summary and refresh icon button;
- one search input for IP, numeric user ID, username, or display name;
- a paginated blocked-IP table;
- expandable account details;
- an Unblock IP action with confirmation that only eligible auto-disabled
  accounts are restored and the count is reset;
- an allowlist table with an exact-IP input, Add command, and Remove actions;
- a stronger confirmation when allowlisting a blocked IP because eligible
  accounts will also be restored.

The tables provide loading, cached refresh, empty, error, mutation-pending, and
pagination states with stable dimensions. Long IPv6 addresses and names wrap or
truncate with tooltips. Familiar icons are used for refresh and disclosure,
with accessible labels.

All new copy uses literal English-source translation keys and the project's
script-only i18n workflow for every locale currently supported by web/default.

## Error Handling and Auditability

- A blocked IP receives a localized error stating that an administrator must
  unblock it.
- The fourth registration receives a distinct localized message stating that
  its account was created but disabled after exceeding the IP account limit.
- IP normalization or storage failure rejects registration rather than silently
  bypassing protection.
- A failed transaction leaves no user, IP association, count increment, partial
  group disable, or provider binding.
- Unblock and allowlist transactions update the state and every eligible account
  together or update nothing.
- Administrator mutations use existing permission middleware and audit helpers.
- Registration-protection events also create server warning logs with canonical
  IP and affected user IDs, never credentials or tokens.

## Testing

Backend behavior tests cover:

- IP canonicalization, IPv4-in-IPv6 unmapping, and invalid input;
- the first three self-service accounts remaining enabled;
- the fourth account being created while all four become disabled and the IP
  becomes blocked;
- later registration rejection without user creation;
- concurrent threshold registrations preserving the invariant;
- password, built-in/custom OAuth, Telegram, and WeChat using the shared path;
- administrator/setup creation and historical accounts being excluded;
- administrator disable clearing restoration eligibility;
- unblock restoring only eligible non-deleted users and beginning a zero-count
  cycle;
- exact-IP allowlist add/remove, invalid and CIDR rejection, and allowlisting a
  blocked IP restoring eligible users;
- blocked-list search by IP, user ID, username, and display name;
- authorization and mutation audit logs;
- portable migrations and queries for SQLite, MySQL, and PostgreSQL;
- affiliate approval creating both the existing administrator audit and the
  user-owned management log only after success;
- failed or duplicate approvals producing no user-owned approval log.

Frontend tests cover search, account disclosure, pagination, refresh, unblock
confirmation, allowlist validation and mutations, blocked-IP allowlist
confirmation, empty/error states, and localized approval-log rendering.

Verification includes focused red-green tests, affected Go packages, the full
Go suite, frontend tests, type checking, lint and format checks on changed files,
i18n synchronization and report checks, a production build, and browser checks
at desktop and mobile widths.

## Non-Goals

- Backfilling IPs for historical users or inferring them from logs.
- CIDR or hostname allowlist rules.
- Device fingerprinting, CAPTCHA changes, or email-domain limits.
- Automatically deleting accounts or reclaiming balances, tokens, or rewards.
- Making the fixed three-account threshold configurable in this iteration.
- Applying the rule to administrator-created or setup users.

## Acceptance Criteria

1. A credited user sees a localized management log with the approved balance
   amount while the existing administrator audit remains.
2. The first three self-service accounts from one exact IP stay enabled.
3. The fourth account is created and all four current-cycle accounts are
   disabled atomically; later registrations from that IP are rejected.
4. All self-service mechanisms use the rule, while administrator, setup, and
   historical accounts do not.
5. An administrator can find a blocked IP through IP or associated-user search,
   inspect its accounts, and unblock it.
6. Unblocking restores only eligible automatically disabled accounts, preserves
   administrator disables, and restarts counting from zero.
7. Exact-IP allowlist entries bypass counting and can be managed safely.
8. The implementation remains portable across SQLite, MySQL, and PostgreSQL and
   passes backend, frontend, i18n, build, and browser verification.
