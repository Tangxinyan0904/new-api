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
import { describe, expect, test } from 'bun:test'

import { formatTimestamp } from '@/lib/format'

import { getInvitedUserAuditPresentation } from '../invited-user-display'

describe('getInvitedUserAuditPresentation', () => {
  test('uses the display-name fallback and preserves registration and login times', () => {
    expect(
      getInvitedUserAuditPresentation({
        id: 1,
        username: 'invitee',
        display_name: '',
        created_at: 1_700_000_000,
        last_login_at: 1_700_000_100,
        is_deleted: false,
      })
    ).toEqual({
      displayName: 'invitee',
      createdAt: formatTimestamp(1_700_000_000),
      lastLoginAt: formatTimestamp(1_700_000_100),
      isDeleted: false,
    })
  })

  test('keeps a deleted user visible when its profile fields and login are absent', () => {
    expect(
      getInvitedUserAuditPresentation({
        id: 2,
        username: '',
        display_name: '',
        created_at: 1_700_000_000,
        last_login_at: 0,
        is_deleted: true,
      })
    ).toEqual({
      displayName: '***',
      createdAt: formatTimestamp(1_700_000_000),
      lastLoginAt: '-',
      isDeleted: true,
    })
  })
})
