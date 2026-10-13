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

import { aggregateModelTokenUsage } from '../stats'

describe('aggregateModelTokenUsage', () => {
  it('sums every hour of a model and lists the busiest model first', () => {
    expect(
      aggregateModelTokenUsage([
        {
          created_at: 0,
          model_name: 'gpt-4o',
          token_used: 100,
          cache_tokens: 40,
        },
        {
          created_at: 3600,
          model_name: 'claude',
          token_used: 500,
          cache_tokens: 300,
        },
        {
          created_at: 3600,
          model_name: 'gpt-4o',
          token_used: 250,
          cache_tokens: 10,
        },
      ])
    ).toEqual([
      { model: 'claude', tokens: 500, cacheTokens: 300 },
      { model: 'gpt-4o', tokens: 350, cacheTokens: 50 },
    ])
  })

  it('counts rows recorded before cached tokens existed as no cache', () => {
    expect(
      aggregateModelTokenUsage([
        { created_at: 0, model_name: 'gpt-4o', token_used: 100 },
      ])
    ).toEqual([{ model: 'gpt-4o', tokens: 100, cacheTokens: 0 }])
  })

  it('orders models with equal usage by name', () => {
    expect(
      aggregateModelTokenUsage([
        { created_at: 0, model_name: 'b-model', token_used: 10 },
        { created_at: 0, model_name: 'a-model', token_used: 10 },
      ]).map((usage) => usage.model)
    ).toEqual(['a-model', 'b-model'])
  })
})
