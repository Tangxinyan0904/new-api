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
import { expect, mock, test } from 'bun:test'

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

mock.module('../components/rebate-approvals-table', () => ({
  RebateApprovalsTable: () => <div />,
}))

const { RebateApprovals } = await import('../index')

test('renders the rebate approvals title in the active locale', async () => {
  const i18n = createInstance()
  await i18n.init({
    lng: 'fr',
    resources: {
      fr: {
        translation: {
          'Rebate Approvals': 'Approbation des remises',
        },
      },
    },
  })

  const markup = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <RebateApprovals />
    </I18nextProvider>
  )

  expect(markup).toContain('Approbation des remises')
})
