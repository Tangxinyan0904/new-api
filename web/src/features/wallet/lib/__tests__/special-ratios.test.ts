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
  getSpecialRatioSummary,
  getWalletPrimaryGridClass,
} from '../special-ratios'

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

  test('stretches every visible card in multi-column layouts', () => {
    expect(getWalletPrimaryGridClass(false, false)).toContain('grid-cols-1')
    const twoCardClasses = getWalletPrimaryGridClass(true, false)
    const threeCardClasses = getWalletPrimaryGridClass(true, true)

    expect(twoCardClasses).toContain('xl:grid-cols-2')
    expect(twoCardClasses).toContain('xl:items-stretch')
    expect(twoCardClasses).not.toContain('items-start')
    expect(threeCardClasses).toContain('2xl:grid-cols-3')
    expect(threeCardClasses).toContain('2xl:items-stretch')
    expect(threeCardClasses).not.toContain('items-start')
  })

  test('describes the current ratio and the upgrade target ratio', () => {
    expect(
      getSpecialRatioSummary({
        billingGroup: 'svip',
        baseRatio: 1,
        specialRatio: 0.8,
      })
    ).toEqual({
      currentRatio: 1,
      upgradeGroup: 'svip',
      upgradeRatio: 0.8,
    })
  })
})
