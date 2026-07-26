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

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

import { RebateInvitedUserList } from './rebate-invited-user-list'

async function renderInvitedUserList(
  users: React.ComponentProps<typeof RebateInvitedUserList>['users']
): Promise<string> {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: {
      en: {
        translation: {
          'Invited User Details': 'Invited User Details',
          'No invited users found.': 'No invited users found.',
          'User ID': 'User ID',
          'Created At': 'Created At',
          'Last Login': 'Last Login',
          Deleted: 'Deleted',
        },
      },
    },
  })

  return renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <RebateInvitedUserList users={users} />
    </I18nextProvider>
  )
}

describe('RebateInvitedUserList', () => {
  test('renders registration, login, fallback, and deleted status for every user', async () => {
    const markup = await renderInvitedUserList([
      {
        id: 11,
        username: 'active-user',
        display_name: 'Active User',
        created_at: 1_700_000_000,
        last_login_at: 1_700_000_100,
        is_deleted: false,
      },
      {
        id: 12,
        username: 'deleted-user',
        display_name: '',
        created_at: 1_700_000_200,
        last_login_at: 0,
        is_deleted: true,
      },
    ])

    assert.match(markup, /Invited User Details/)
    assert.match(markup, /Active User/)
    assert.match(markup, /deleted-user/)
    assert.match(markup, /User ID/)
    assert.match(markup, /Created At/)
    assert.match(markup, /Last Login/)
    assert.match(markup, />-</)
    assert.equal(markup.match(/Deleted/g)?.length, 1)
  })

  test('renders the empty state when there are no invited users', async () => {
    const markup = await renderInvitedUserList([])

    assert.match(markup, /No invited users found\./)
  })
})
