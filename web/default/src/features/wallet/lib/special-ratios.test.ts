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

import {
  getSpecialRatioCardState,
  getWalletPrimaryGridClass,
} from './special-ratios'

describe('wallet special ratio card', () => {
  test('keeps loading and errors visible but hides a successful empty result', () => {
    expect(
      getSpecialRatioCardState({
        isPending: true,
        isError: false,
        count: 0,
      })
    ).toEqual({ display: 'loading', available: true })
    expect(
      getSpecialRatioCardState({
        isPending: false,
        isError: true,
        count: 0,
      })
    ).toEqual({ display: 'error', available: true })
    expect(
      getSpecialRatioCardState({
        isPending: false,
        isError: false,
        count: 0,
      })
    ).toEqual({ display: 'hidden', available: false })
    expect(
      getSpecialRatioCardState({
        isPending: false,
        isError: false,
        count: 2,
      })
    ).toEqual({ display: 'rules', available: true })
  })

  test('selects one, two, and three-card grid classes', () => {
    expect(getWalletPrimaryGridClass(false, false)).toContain('grid-cols-1')
    expect(getWalletPrimaryGridClass(true, false)).toContain('xl:grid-cols-2')
    expect(getWalletPrimaryGridClass(false, true)).toContain('xl:grid-cols-2')
    expect(getWalletPrimaryGridClass(true, true)).toContain('2xl:grid-cols-3')
  })
})
