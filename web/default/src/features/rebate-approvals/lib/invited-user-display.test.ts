/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { formatTimestamp } from '@/lib/format'

import { getInvitedUserAuditPresentation } from './invited-user-display'

describe('invited user display', () => {
  test('formats active users with display-name fallback and timestamps', () => {
    assert.deepEqual(
      getInvitedUserAuditPresentation({
        id: 1,
        username: 'user',
        display_name: '',
        created_at: 1_700_000_000,
        last_login_at: 1_700_000_100,
        is_deleted: false,
      }),
      {
        displayName: 'user',
        createdAt: formatTimestamp(1_700_000_000),
        lastLoginAt: formatTimestamp(1_700_000_100),
        isDeleted: false,
      }
    )
  })

  test('shows a neutral name and dash for a deleted user who never logged in', () => {
    assert.deepEqual(
      getInvitedUserAuditPresentation({
        id: 2,
        username: '',
        display_name: '',
        created_at: 1_700_000_000,
        last_login_at: 0,
        is_deleted: true,
      }),
      {
        displayName: '***',
        createdAt: formatTimestamp(1_700_000_000),
        lastLoginAt: '-',
        isDeleted: true,
      }
    )
  })
})
