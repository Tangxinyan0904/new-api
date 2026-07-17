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

import {
  normalizeVerificationEmail,
  shouldShowVerificationEmailReminder,
} from './email-verification-reminder'

describe('verification email reminder', () => {
  test('normalizes surrounding spaces and letter case', () => {
    assert.equal(
      normalizeVerificationEmail(' User@Example.COM '),
      'user@example.com'
    )
  })

  test('shows only for the latest successfully sent address', () => {
    assert.equal(
      shouldShowVerificationEmailReminder(
        ' User@Example.com ',
        'user@example.com'
      ),
      true
    )
    assert.equal(
      shouldShowVerificationEmailReminder(
        'other@example.com',
        'user@example.com'
      ),
      false
    )
  })

  test('stays hidden until an email has been sent successfully', () => {
    assert.equal(
      shouldShowVerificationEmailReminder('user@example.com', null),
      false
    )
  })
})
