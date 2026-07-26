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
import * as z from 'zod'

import type {
  HydratedUserRateLimitRule,
  RateLimitUserSummary,
  UserRateLimitRule,
} from './types'

export const MAX_RATE_LIMIT_VALUE = 2147483647

type RateLimitPair = [number, number]
type Translate = (key: string) => string

function parseRateLimitPairRecord(
  value: string
): Record<string, RateLimitPair> {
  const source = value.trim() || '{}'
  const parsed: unknown = JSON.parse(source)
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error('Rate limits must be a JSON object')
  }

  const limits: Record<string, RateLimitPair> = {}
  for (const [key, pair] of Object.entries(parsed)) {
    if (
      !Array.isArray(pair) ||
      pair.length !== 2 ||
      !Number.isInteger(pair[0]) ||
      !Number.isInteger(pair[1]) ||
      pair[0] < 0 ||
      pair[1] < 1 ||
      pair[0] > MAX_RATE_LIMIT_VALUE ||
      pair[1] > MAX_RATE_LIMIT_VALUE
    ) {
      throw new Error(`Invalid rate limit values for ${key}`)
    }
    limits[key] = [pair[0], pair[1]]
  }
  return limits
}

export function parseUserRateLimitRules(value: string): UserRateLimitRule[] {
  const limits = parseRateLimitPairRecord(value)
  const rules = Object.entries(limits).map(([userIdText, pair]) => {
    const userId = Number(userIdText)
    if (
      !Number.isSafeInteger(userId) ||
      userId <= 0 ||
      String(userId) !== userIdText
    ) {
      throw new Error(`Invalid user ID ${userIdText}`)
    }
    return {
      userId,
      maxRequests: pair[0],
      maxSuccess: pair[1],
    }
  })
  return rules.sort((left, right) => left.userId - right.userId)
}

export function serializeUserRateLimitRules(
  rules: UserRateLimitRule[]
): string {
  const limits: Record<string, RateLimitPair> = {}
  const sortedRules = [...rules].sort(
    (left, right) => left.userId - right.userId
  )
  for (const rule of sortedRules) {
    limits[String(rule.userId)] = [rule.maxRequests, rule.maxSuccess]
  }
  return JSON.stringify(limits)
}

export function addUserRateLimitRule(
  rules: UserRateLimitRule[],
  candidate: UserRateLimitRule
): { rules: UserRateLimitRule[]; added: boolean } {
  if (rules.some((rule) => rule.userId === candidate.userId)) {
    return { rules, added: false }
  }
  return {
    rules: [...rules, candidate].sort(
      (left, right) => left.userId - right.userId
    ),
    added: true,
  }
}

export function hydrateUserRateLimitRules(
  rules: UserRateLimitRule[],
  users: RateLimitUserSummary[]
): HydratedUserRateLimitRule[] {
  const usersById = new Map(users.map((user) => [user.id, user]))
  return rules.map((rule) => {
    const user = usersById.get(rule.userId)
    if (!user) return rule
    return {
      ...rule,
      username: user.username,
      displayName: user.displayName,
    }
  })
}

export function createRateLimitSchema(t: Translate) {
  const integerFromZero = z.number().int().min(0).max(MAX_RATE_LIMIT_VALUE)
  const positiveInteger = z.number().int().min(1).max(MAX_RATE_LIMIT_VALUE)
  const jsonRateLimits = z.string().superRefine((value, context) => {
    try {
      parseRateLimitPairRecord(value)
    } catch {
      context.addIssue({
        code: 'custom',
        message: t('Invalid JSON format or values out of allowed range'),
      })
    }
  })
  const userRateLimits = z.string().superRefine((value, context) => {
    try {
      parseUserRateLimitRules(value)
    } catch {
      context.addIssue({
        code: 'custom',
        message: t('User rate limits must use positive numeric user IDs'),
      })
    }
  })

  return z
    .object({
      ModelRequestRateLimitEnabled: z.boolean(),
      ModelRequestRateLimitDurationMinutes: integerFromZero,
      ModelRequestRateLimitCount: integerFromZero,
      ModelRequestRateLimitSuccessCount: positiveInteger,
      ModelRequestRateLimitGroup: jsonRateLimits,
      ModelRequestRateLimitUser: userRateLimits,
      ModelRequestRateLimitDistillationEnabled: z.boolean(),
      ModelRequestRateLimitDistillationThreshold: integerFromZero,
      ModelRequestRateLimitDistillationRPM: integerFromZero,
      ModelRequestRateLimitDistillationPenaltyMinutes: integerFromZero,
      ModelRequestRateLimitDistillationObservationMinutes: integerFromZero,
    })
    .superRefine((values, context) => {
      if (!values.ModelRequestRateLimitDistillationEnabled) return

      const positiveFields = [
        'ModelRequestRateLimitDistillationThreshold',
        'ModelRequestRateLimitDistillationRPM',
        'ModelRequestRateLimitDistillationPenaltyMinutes',
        'ModelRequestRateLimitDistillationObservationMinutes',
      ] as const
      for (const field of positiveFields) {
        if (values[field] > 0) continue
        context.addIssue({
          code: 'custom',
          path: [field],
          message: t(
            'Distillation settings must be greater than 0 when detection is enabled'
          ),
        })
      }

      if (
        values.ModelRequestRateLimitDistillationRPM >=
        values.ModelRequestRateLimitDistillationThreshold
      ) {
        context.addIssue({
          code: 'custom',
          path: ['ModelRequestRateLimitDistillationRPM'],
          message: t(
            'Punishment RPM must be lower than the detection threshold'
          ),
        })
      }
    })
}
