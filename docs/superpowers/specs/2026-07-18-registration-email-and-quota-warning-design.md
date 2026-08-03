# Registration Email and Low-Balance Warning Design

Date: 2026-07-18

## Goal

Improve two email-related user flows:

1. After a registration verification email is sent successfully, keep a prominent reminder visible so the user knows to check the spam or junk folder.
2. Send a wallet low-balance email only once during each continuous period below the configured threshold, then allow another email only after the wallet balance has recovered to or above that threshold.

## Confirmed Decisions

- The registration reminder is persistent inline text, not a second toast.
- The reminder uses the existing destructive alert styling so its red text and border remain readable on both light and dark backgrounds.
- The reminder appears only after a verification email has been sent successfully.
- Changing the email address hides the old reminder. A successful send to the new address shows it again.
- The existing successful-send toast and resend countdown remain unchanged.
- The low-balance change applies only to wallet notifications whose configured notification method is email.
- Webhook, Bark, Gotify, and subscription-quota notifications retain their current behavior.
- No database table or column is added. The low-balance notification latch is stored as internal state in the user's existing `setting` JSON.

## Registration Reminder

The email-verification hook exposes the email address for which the most recent send succeeded. The sign-up form compares that value with the current normalized email input.

When they match, a compact destructive `Alert` is rendered immediately below the verification-code input and send button. It tells the user that if the message is not visible in the inbox, it may have been intercepted by the email provider and should be checked in the spam or junk folder.

The reminder is hidden before the first successful send and whenever the email input no longer matches the successfully sent address. A failed initial send does not show it. A failed resend does not erase an earlier successful-send reminder for the same address. Editing the address back to the same value may show the reminder again because that address has already received a successful send in the current form session.

All reminder text uses the default frontend i18n system and is translated for every runtime locale.

## Low-Balance Email State

### Storage

Persist an internal boolean latch in the existing `users.setting` JSON. The latch is not accepted from the user-settings API and must be preserved when the user changes ordinary notification settings. Existing settings without the internal key are treated as not notified.

The implementation must use `common.Marshal` and `common.Unmarshal` for this JSON. It must not add a database column, table, or migration.

### State Transitions

For the effective threshold, continue using the user's `quota_warning_threshold` when nonzero and the global `QuotaRemindThreshold` otherwise.

The wallet email state has two logical states:

- `armed`: no successful low-balance email has been recorded for the current below-threshold period;
- `notified`: a low-balance email has already been sent for that period.

The transitions are:

1. If the wallet balance is below the threshold and the state is `armed`, atomically claim the notification, send one email, and move to `notified`.
2. If the wallet remains below the threshold while `notified`, do not send another low-balance email.
3. When the wallet balance reaches or exceeds the threshold, move back to `armed`.
4. A later drop below the threshold may then send one new email.
5. If the email send fails, release the claimed state so a later request can retry.

State reads and writes are serialized through the existing user row and `lockForUpdate(tx)` where supported. The transaction updates only the `setting` column and preserves all public and unrelated internal setting values. Cache state is refreshed only after a successful commit.

Balance recovery must rearm the latch from every wallet-credit path used by normal product flows, including recharge, redemption, check-in rewards, affiliate transfers, administrator quota increases, and billing refunds. The rearm helper checks the resulting balance against the user's effective threshold; a partial credit that remains below the threshold does not rearm the latch.

### Notification Scope

Only `dto.NotifyTypeEmail` uses the latch. Other configured methods continue through the existing notification-rate limit and delivery behavior. The existing subscription quota warning function is unchanged.

The low-balance email displays the calculated balance after the completed wallet adjustment rather than a stale pre-consumption balance.

## Failure and Concurrency Handling

- A state-claim failure is logged and skips delivery rather than risking duplicate email.
- A delivery failure is logged and releases the latch for a later retry.
- Concurrent requests for one user cannot both claim the same armed state.
- A recovery update and a warning claim are serialized on the same user setting record.
- Existing malformed setting JSON makes the latch operation fail safely and log the error; the malformed value must not be overwritten.
- Cache-update failures are logged after the database state remains authoritative.

## Testing

### Backend

Add deterministic tests for these observable contracts:

- the first wallet request below the threshold claims one email notification;
- additional requests while still below the threshold do not claim another;
- a partial credit that remains below the threshold does not rearm;
- recovery to exactly the threshold rearms;
- recovery above the threshold rearms;
- a second drop after recovery claims one new notification;
- a failed email delivery releases the latch for retry;
- concurrent claims yield at most one winner;
- updating public user settings preserves the internal latch;
- non-email and subscription notifications retain their existing behavior.

New Go tests use `testify/require` for setup and fatal assertions and `testify/assert` for non-fatal value checks.

### Frontend

Add focused tests for the registration flow:

- the spam-folder reminder is absent before a successful send;
- it appears after a successful send;
- a failed initial send does not display it, while a failed resend preserves an earlier reminder for the same address;
- changing the email hides it;
- the reminder uses the destructive alert presentation.

Run the focused frontend tests, i18n synchronization checks, type checking, and the production frontend build.

## Out of Scope

- Database schema changes or migrations.
- Changing the configured low-balance threshold.
- Changing notification-method settings or delivery templates beyond using the post-adjustment balance.
- Applying the latch to Webhook, Bark, Gotify, or subscription-quota warnings.
- Changing the general notification rate limiter.
