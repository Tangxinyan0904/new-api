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
export type RateLimitFormValues = {
  ModelRequestRateLimitEnabled: boolean
  ModelRequestRateLimitDurationMinutes: number
  ModelRequestRateLimitCount: number
  ModelRequestRateLimitSuccessCount: number
  ModelRequestRateLimitGroup: string
  ModelRequestRateLimitUser: string
  ModelRequestRateLimitDistillationEnabled: boolean
  ModelRequestRateLimitDistillationThreshold: number
  ModelRequestRateLimitDistillationRPM: number
  ModelRequestRateLimitDistillationPenaltyMinutes: number
  ModelRequestRateLimitDistillationObservationMinutes: number
}

export type UserRateLimitRule = {
  userId: number
  maxRequests: number
  maxSuccess: number
}

export type RateLimitUserSummary = {
  id: number
  username: string
  displayName?: string
}

export type HydratedUserRateLimitRule = UserRateLimitRule & {
  username?: string
  displayName?: string
}

export type DistillationPenaltyPhase = 'temporary' | 'observation' | 'permanent'

export type DistillationPenalty = {
  user_id: number
  username: string
  phase: DistillationPenaltyPhase
  first_triggered_at: number
  temporary_limited_until: number
  observation_until: number
  permanently_banned_at: number
  created_at: number
  updated_at: number
}

export type PageData<T> = {
  page: number
  page_size: number
  total: number
  items: T[]
}

export type RateLimitApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}
