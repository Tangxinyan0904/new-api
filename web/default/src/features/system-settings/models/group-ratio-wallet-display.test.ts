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
  buildWalletDisplayOptionUpdates,
  moveWalletDisplayRule,
  parseWalletDisplayMap,
  setWalletDisplayRule,
  validateWalletDisplayPairs,
} from './group-ratio-wallet-display'

describe('wallet special-ratio display settings', () => {
  test('selects, moves, and removes one pair without stale keys', () => {
    const selected = setWalletDisplayRule('{}', 'vip', 'premium', true)
    expect(parseWalletDisplayMap(selected)).toEqual({
      vip: { premium: true },
    })

    const moved = moveWalletDisplayRule(
      selected,
      'vip',
      'premium',
      'vip',
      'pro'
    )
    expect(parseWalletDisplayMap(moved)).toEqual({ vip: { pro: true } })
    expect(
      parseWalletDisplayMap(setWalletDisplayRule(moved, 'vip', 'pro', false))
    ).toEqual({})
  })

  test('rejects selected pairs absent from the ratio map', () => {
    expect(() =>
      validateWalletDisplayPairs(
        '{"vip":{"missing":true}}',
        '{"vip":{"premium":0.3}}'
      )
    ).toThrow('vip -> missing')
  })

  test('orders ratio update before wallet display update', () => {
    expect(
      buildWalletDisplayOptionUpdates(
        {
          GroupGroupRatio: '{"vip":{"premium":0.3}}',
          GroupGroupRatioWalletDisplay: '{"vip":{"premium":true}}',
        },
        { GroupGroupRatio: '{}', GroupGroupRatioWalletDisplay: '{}' }
      ).map((item) => item.key)
    ).toEqual([
      'GroupGroupRatio',
      'group_ratio_setting.group_group_ratio_wallet_display',
    ])
  })
})
