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

import { getCacheHitMetrics } from './cache-metrics'

describe('getCacheHitMetrics', () => {
  test('uses OpenAI input tokens that already include cache reads', () => {
    assert.deepEqual(getCacheHitMetrics(1000, { cache_tokens: 250 }), {
      cacheReadTokens: 250,
      cacheWriteTokens: 0,
      totalInputTokens: 1000,
      percentage: 25,
      formattedPercentage: '25.000%',
    })
  })

  test('uses separate Claude input, cache read, and cache write tokens', () => {
    assert.deepEqual(
      getCacheHitMetrics(100, {
        usage_semantic: 'anthropic',
        cache_tokens: 300,
        cache_write_tokens: 100,
      }),
      {
        cacheReadTokens: 300,
        cacheWriteTokens: 100,
        totalInputTokens: 500,
        percentage: 60,
        formattedPercentage: '60.000%',
      }
    )
  })

  test('prefers normalized Claude cache writes over legacy fields', () => {
    const metrics = getCacheHitMetrics(100, {
      usage_semantic: 'anthropic',
      cache_tokens: 200,
      cache_write_tokens: 50,
      cache_creation_tokens: 500,
      cache_creation_tokens_5m: 20,
      cache_creation_tokens_1h: 30,
    })

    assert.equal(metrics.cacheWriteTokens, 50)
    assert.equal(metrics.totalInputTokens, 350)
  })

  test('sums split Claude cache writes without double counting', () => {
    const metrics = getCacheHitMetrics(100, {
      usage_semantic: 'anthropic',
      cache_tokens: 200,
      cache_creation_tokens: 500,
      cache_creation_tokens_5m: 20,
      cache_creation_tokens_1h: 30,
    })

    assert.equal(metrics.cacheWriteTokens, 50)
    assert.equal(metrics.totalInputTokens, 350)
  })

  test('falls back to unsplit cache creation for legacy Claude logs', () => {
    const metrics = getCacheHitMetrics(100, {
      claude: true,
      cache_tokens: 200,
      cache_creation_tokens: 50,
    })

    assert.equal(metrics.cacheWriteTokens, 50)
    assert.equal(metrics.totalInputTokens, 350)
    assert.equal(metrics.formattedPercentage, '57.143%')
  })

  test('returns zero for requests without input or cache tokens', () => {
    assert.deepEqual(getCacheHitMetrics(0, {}), {
      cacheReadTokens: 0,
      cacheWriteTokens: 0,
      totalInputTokens: 0,
      percentage: 0,
      formattedPercentage: '0.000%',
    })
  })

  test('normalizes negative and non-finite historical values', () => {
    assert.deepEqual(
      getCacheHitMetrics(Number.NaN, {
        usage_semantic: 'anthropic',
        cache_tokens: Number.POSITIVE_INFINITY,
        cache_write_tokens: -1,
      }),
      {
        cacheReadTokens: 0,
        cacheWriteTokens: 0,
        totalInputTokens: 0,
        percentage: 0,
        formattedPercentage: '0.000%',
      }
    )
  })

  test('clamps malformed OpenAI cache ratios to one hundred percent', () => {
    assert.equal(
      getCacheHitMetrics(100, { cache_tokens: 200 }).formattedPercentage,
      '100.000%'
    )
  })
})
