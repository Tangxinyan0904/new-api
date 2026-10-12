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
import { describe, expect, test } from 'vitest'

import { getChannelMonitorFormSchema } from '../schema'

const VALID_INPUT = {
  internal_name: 'Primary OpenAI monitor',
  public_name: 'OpenAI',
  name_color: '',
  provider: 'openai',
  api_mode: 'chat_completions',
  endpoint: 'https://api.example.com',
  api_key: 'monitor-secret',
  primary_model: 'model-primary',
  extra_models: [],
  interval_seconds: 60,
  degraded_threshold_ms: 6000,
  timeout_seconds: 30,
  enabled: true,
  public_enabled: false,
}

describe('channel monitor form schema', () => {
  test('accepts empty name color and normalizes exact hex colors', () => {
    const schema = getChannelMonitorFormSchema(false)

    for (const [nameColor, expected] of [
      ['', ''],
      ['#1A2B3C', '#1A2B3C'],
      ['  #abcdef  ', '#ABCDEF'],
    ] as const) {
      const result = schema.safeParse({ ...VALID_INPUT, name_color: nameColor })
      expect(result.success).toBe(true)
      if (!result.success) continue
      expect(result.data.name_color).toBe(expected)
    }
  })

  test('rejects malformed name colors', () => {
    const schema = getChannelMonitorFormSchema(false)

    for (const nameColor of ['112233', '#123', '#12345G', '#1234567']) {
      expect(
        schema.safeParse({ ...VALID_INPUT, name_color: nameColor }).success
      ).toBe(false)
    }
  })

  test('accepts only the four supported provider and API mode pairs', () => {
    const schema = getChannelMonitorFormSchema(false)
    const validPairs = [
      ['openai', 'chat_completions'],
      ['openai', 'responses'],
      ['anthropic', 'messages'],
      ['gemini', 'generate_content'],
    ] as const

    for (const [provider, apiMode] of validPairs) {
      expect(
        schema.safeParse({
          ...VALID_INPUT,
          provider,
          api_mode: apiMode,
        }).success
      ).toBe(true)
    }

    expect(
      schema.safeParse({
        ...VALID_INPUT,
        provider: 'anthropic',
        api_mode: 'responses',
      }).success
    ).toBe(false)
  })

  test('rejects duplicate models and more than twenty total models', () => {
    const schema = getChannelMonitorFormSchema(false)

    expect(
      schema.safeParse({
        ...VALID_INPUT,
        extra_models: ['model-primary'],
      }).success
    ).toBe(false)
    expect(
      schema.safeParse({
        ...VALID_INPUT,
        extra_models: Array.from(
          { length: 19 },
          (_, index) => `model-${index}`
        ),
      }).success
    ).toBe(true)
    expect(
      schema.safeParse({
        ...VALID_INPUT,
        extra_models: Array.from(
          { length: 20 },
          (_, index) => `model-${index}`
        ),
      }).success
    ).toBe(false)
  })

  test('enforces interval and timeout boundaries', () => {
    const schema = getChannelMonitorFormSchema(false)

    expect(
      schema.safeParse({ ...VALID_INPUT, interval_seconds: 59 }).success
    ).toBe(false)
    expect(
      schema.safeParse({ ...VALID_INPUT, interval_seconds: 3600 }).success
    ).toBe(true)
    expect(
      schema.safeParse({ ...VALID_INPUT, interval_seconds: 3601 }).success
    ).toBe(false)
    expect(
      schema.safeParse({ ...VALID_INPUT, timeout_seconds: 0 }).success
    ).toBe(false)
    expect(
      schema.safeParse({ ...VALID_INPUT, timeout_seconds: 120 }).success
    ).toBe(true)
    expect(
      schema.safeParse({ ...VALID_INPUT, timeout_seconds: 121 }).success
    ).toBe(false)
  })

  test('rejects endpoints longer than the backend limit', () => {
    const result = getChannelMonitorFormSchema(false).safeParse({
      ...VALID_INPUT,
      endpoint: `https://api.example.com/${'a'.repeat(2048)}`,
    })

    expect(result.success).toBe(false)
  })

  test('returns a translatable error for an oversized extra model name', () => {
    const result = getChannelMonitorFormSchema(false).safeParse({
      ...VALID_INPUT,
      extra_models: ['a'.repeat(257)],
    })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.message).toBe('Extra model is too long')
  })

  test('requires a credential on create and allows an empty replacement on edit', () => {
    expect(
      getChannelMonitorFormSchema(false).safeParse({
        ...VALID_INPUT,
        api_key: '',
      }).success
    ).toBe(false)
    expect(
      getChannelMonitorFormSchema(true).safeParse({
        ...VALID_INPUT,
        api_key: '',
      }).success
    ).toBe(true)
  })
})
