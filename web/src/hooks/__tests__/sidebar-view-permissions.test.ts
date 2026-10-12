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
import { describe, expect, it } from 'vitest'

import type { NavGroup } from '@/components/layout/types'
import { ROLE } from '@/lib/roles'
import type { AuthUser } from '@/stores/auth-store'

import { filterNavGroupsForUser } from '../use-sidebar-view'

const groups: NavGroup[] = [
  {
    id: 'personal',
    title: 'Personal',
    items: [{ title: 'Wallet', url: '/wallet' }],
  },
  {
    id: 'admin',
    title: 'Admin',
    items: [
      { title: 'Channels', url: '/channels' },
      {
        title: 'Cost Statistics',
        url: '/channel-costs',
        requiredPermission: { resource: 'channel_cost', action: 'read' },
      },
      {
        title: 'System Info',
        url: '/system-info',
        requiredRole: ROLE.SUPER_ADMIN,
      },
    ],
  },
]

function titles(user: AuthUser | null): string[] {
  return filterNavGroupsForUser(groups, user).flatMap((group) =>
    group.items.map((item) => item.title)
  )
}

function admin(
  permissions?: Record<string, Record<string, boolean>>
): AuthUser {
  return {
    id: 2,
    username: 'admin',
    role: ROLE.ADMIN,
    permissions: permissions ? { admin_permissions: permissions } : undefined,
  } as AuthUser
}

describe('filterNavGroupsForUser', () => {
  it('hides permission-gated items from admins without the permission', () => {
    expect(titles(admin())).toEqual(['Wallet', 'Channels'])
    expect(titles(admin({ channel_cost: { read: false } }))).toEqual([
      'Wallet',
      'Channels',
    ])
  })

  it('shows permission-gated items to admins granted the permission', () => {
    expect(titles(admin({ channel_cost: { read: true } }))).toEqual([
      'Wallet',
      'Channels',
      'Cost Statistics',
    ])
  })

  it('shows every item to the super admin', () => {
    expect(
      titles({ id: 1, username: 'root', role: ROLE.SUPER_ADMIN } as AuthUser)
    ).toEqual(['Wallet', 'Channels', 'Cost Statistics', 'System Info'])
  })

  it('hides the admin group from regular users', () => {
    expect(
      titles({ id: 3, username: 'alice', role: ROLE.USER } as AuthUser)
    ).toEqual(['Wallet'])
  })
})
