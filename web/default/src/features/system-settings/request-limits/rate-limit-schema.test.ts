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
  addUserRateLimitRule,
  createRateLimitSchema,
  hydrateUserRateLimitRules,
  parseUserRateLimitRules,
  serializeUserRateLimitRules,
} from './rate-limit-schema'

const translate = (key: string) => key

const validFormValues = {
  ModelRequestRateLimitEnabled: true,
  ModelRequestRateLimitDurationMinutes: 1,
  ModelRequestRateLimitCount: 100,
  ModelRequestRateLimitSuccessCount: 80,
  ModelRequestRateLimitGroup: '{}',
  ModelRequestRateLimitUser: '{}',
  ModelRequestRateLimitDistillationEnabled: true,
  ModelRequestRateLimitDistillationThreshold: 60,
  ModelRequestRateLimitDistillationRPM: 10,
  ModelRequestRateLimitDistillationPenaltyMinutes: 30,
  ModelRequestRateLimitDistillationObservationMinutes: 1440,
}

describe('user rate limit JSON helpers', () => {
  test('parses canonical positive user IDs into sorted rules', () => {
    expect(parseUserRateLimitRules('{"42":[20,10],"7":[0,5]}')).toEqual([
      { userId: 7, maxRequests: 0, maxSuccess: 5 },
      { userId: 42, maxRequests: 20, maxSuccess: 10 },
    ])
    expect(() => parseUserRateLimitRules('{"01":[20,10]}')).toThrow()
  })

  test('serializes rules with stable numeric ID ordering', () => {
    expect(
      serializeUserRateLimitRules([
        { userId: 10, maxRequests: 100, maxSuccess: 50 },
        { userId: 2, maxRequests: 20, maxSuccess: 10 },
      ])
    ).toBe('{"2":[20,10],"10":[100,50]}')
  })

  test('does not add a duplicate configured user', () => {
    const existing = [{ userId: 7, maxRequests: 20, maxSuccess: 10 }]
    const result = addUserRateLimitRule(existing, {
      userId: 7,
      maxRequests: 30,
      maxSuccess: 15,
    })

    expect(result.added).toBe(false)
    expect(result.rules).toEqual(existing)
  })

  test('preserves configured IDs that are absent from user search results', () => {
    const hydrated = hydrateUserRateLimitRules(
      [
        { userId: 7, maxRequests: 20, maxSuccess: 10 },
        { userId: 99, maxRequests: 30, maxSuccess: 15 },
      ],
      [{ id: 7, username: 'alice', displayName: 'Alice' }]
    )

    expect(hydrated).toEqual([
      {
        userId: 7,
        username: 'alice',
        displayName: 'Alice',
        maxRequests: 20,
        maxSuccess: 10,
      },
      { userId: 99, maxRequests: 30, maxSuccess: 15 },
    ])
  })
})

describe('rate limit form schema', () => {
  test('allows zero distillation values while detection is disabled', () => {
    const result = createRateLimitSchema(translate).safeParse({
      ...validFormValues,
      ModelRequestRateLimitDistillationEnabled: false,
      ModelRequestRateLimitDistillationThreshold: 0,
      ModelRequestRateLimitDistillationRPM: 0,
      ModelRequestRateLimitDistillationPenaltyMinutes: 0,
      ModelRequestRateLimitDistillationObservationMinutes: 0,
    })

    expect(result.success).toBe(true)
  })

  test('requires punishment RPM below the enabled detection threshold', () => {
    const schema = createRateLimitSchema(translate)

    expect(schema.safeParse(validFormValues).success).toBe(true)
    expect(
      schema.safeParse({
        ...validFormValues,
        ModelRequestRateLimitDistillationRPM: 60,
      }).success
    ).toBe(false)
  })
})
