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
import { describe, expect, mock, test } from 'bun:test'

import { renderToStaticMarkup } from 'react-dom/server'

import type { SystemStatus } from '../types'

mock.module('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const { LegalConsent } = await import('./legal-consent')

const legalStatus = {
  user_agreement_enabled: true,
  privacy_policy_enabled: true,
} as SystemStatus

describe('LegalConsent', () => {
  test('renders a high-contrast unchecked row with a larger checkbox', () => {
    const html = renderToStaticMarkup(
      <LegalConsent
        status={legalStatus}
        checked={false}
        onCheckedChange={() => undefined}
      />
    )

    expect(html).toContain('border-destructive/70')
    expect(html).toContain('bg-destructive/10')
    expect(html).toContain('focus-within:ring-3')
    expect(html).toContain('size-5')
    expect(html).toContain('href="/user-agreement"')
    expect(html).toContain('href="/privacy-policy"')
    expect(html).toContain('and')
  })

  test('renders a distinct confirmed state', () => {
    const html = renderToStaticMarkup(
      <LegalConsent
        status={legalStatus}
        checked
        onCheckedChange={() => undefined}
      />
    )

    expect(html).toContain('border-primary/70')
    expect(html).toContain('bg-primary/10')
    expect(html).not.toContain('border-destructive/70')
  })

  test('renders nothing when no legal document is enabled', () => {
    const html = renderToStaticMarkup(
      <LegalConsent
        status={{} as SystemStatus}
        checked={false}
        onCheckedChange={() => undefined}
      />
    )

    expect(html).toBe('')
  })
})
