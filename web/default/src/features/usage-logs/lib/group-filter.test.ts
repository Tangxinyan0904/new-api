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

import { buildLogGroupOptions } from './group-filter'

describe('usage log group filter options', () => {
  test('sorts configured groups without changing their exact names', () => {
    expect(
      buildLogGroupOptions({
        vip_long_group: { desc: 'VIP', ratio: 0.8 },
        default: { desc: 'Default', ratio: 1 },
        alpha: { desc: 'Alpha', ratio: 0.5 },
      })
    ).toEqual([
      { value: 'alpha', label: 'alpha' },
      { value: 'default', label: 'default' },
      { value: 'vip_long_group', label: 'vip_long_group' },
    ])
  })

  test('returns no suggestions when configured groups are unavailable', () => {
    expect(buildLogGroupOptions(undefined)).toEqual([])
  })
})
