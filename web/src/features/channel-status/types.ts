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
import { z } from 'zod'

export const channelStatusDaysSchema = z.union([
  z.literal(1),
  z.literal(3),
  z.literal(7),
])

export const channelStatusSchema = z.enum([
  'operational',
  'degraded',
  'failed',
  'unknown',
])

export const channelStatusErrorCategorySchema = z.enum([
  '',
  'network',
  'tls',
  'auth',
  'rate_limit',
  'timeout',
  'upstream',
  'invalid_response',
  'config',
])

export const channelStatusProviderSchema = z.enum([
  'openai',
  'anthropic',
  'gemini',
])

export const channelStatusCountsSchema = z.object({
  operational: z.number().int().nonnegative(),
  degraded: z.number().int().nonnegative(),
  failed: z.number().int().nonnegative(),
  unknown: z.number().int().nonnegative(),
})

export const channelStatusTimelinePointSchema = z.object({
  status: channelStatusSchema,
  latency_ms: z.number().int().nonnegative(),
  error_category: channelStatusErrorCategorySchema,
  checked_at: z.number().int().nonnegative(),
})

export const channelStatusItemSchema = z.object({
  id: z.number().int().positive(),
  public_name: z.string(),
  provider: channelStatusProviderSchema,
  primary_model: z.string(),
  status: channelStatusSchema,
  latency_ms: z.number().int().nonnegative().nullable(),
  error_category: channelStatusErrorCategorySchema,
  last_checked_at: z.number().int().nonnegative(),
  availability: z.number().finite().nullable(),
  average_latency_ms: z.number().finite().nonnegative().nullable(),
  timeline: z.array(channelStatusTimelinePointSchema).max(60),
})

export const channelStatusResponseSchema = z.object({
  overall_status: channelStatusSchema,
  last_updated_at: z.number().int().nonnegative(),
  counts: channelStatusCountsSchema,
  days: channelStatusDaysSchema,
  items: z.array(channelStatusItemSchema),
})

export const channelModelStatusSchema = z.object({
  model: z.string(),
  is_primary: z.boolean(),
  status: channelStatusSchema,
  latency_ms: z.number().int().nonnegative().nullable(),
  error_category: channelStatusErrorCategorySchema,
  last_checked_at: z.number().int().nonnegative(),
  availability_1d: z.number().finite().nullable(),
  availability_3d: z.number().finite().nullable(),
  availability_7d: z.number().finite().nullable(),
  average_latency_ms_1d: z.number().finite().nonnegative().nullable(),
  average_latency_ms_3d: z.number().finite().nonnegative().nullable(),
  average_latency_ms_7d: z.number().finite().nonnegative().nullable(),
})

export const channelStatusDetailSchema = z.object({
  id: z.number().int().positive(),
  public_name: z.string(),
  provider: channelStatusProviderSchema,
  primary_model: z.string(),
  status: channelStatusSchema,
  last_updated_at: z.number().int().nonnegative(),
  models: z.array(channelModelStatusSchema),
  timeline: z.array(channelStatusTimelinePointSchema).max(60),
})

export const publicChannelStatusMonitorSchema = z.object({
  id: z.number().int().positive(),
  public_name: z.string(),
  provider: channelStatusProviderSchema,
  primary_model: z.string(),
  status: channelStatusSchema,
  latency_ms: z.number().int().nonnegative().nullable(),
  error_category: channelStatusErrorCategorySchema,
  availability_1d: z.number().finite().nullable(),
  last_checked_at: z.number().int().nonnegative(),
})

export const publicChannelStatusSummarySchema = z.object({
  overall_status: channelStatusSchema,
  last_updated_at: z.number().int().nonnegative(),
  counts: channelStatusCountsSchema,
  monitors: z.array(publicChannelStatusMonitorSchema),
})

export const CHANNEL_STATUS_PROVIDER_LABELS = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
} as const

export type ChannelStatusDays = z.infer<typeof channelStatusDaysSchema>
export type ChannelStatus = z.infer<typeof channelStatusSchema>
export type ChannelStatusErrorCategory = z.infer<
  typeof channelStatusErrorCategorySchema
>
export type ChannelStatusCounts = z.infer<typeof channelStatusCountsSchema>
export type ChannelStatusTimelinePoint = z.infer<
  typeof channelStatusTimelinePointSchema
>
export type ChannelStatusItem = z.infer<typeof channelStatusItemSchema>
export type ChannelStatusResponse = z.infer<typeof channelStatusResponseSchema>
export type ChannelModelStatus = z.infer<typeof channelModelStatusSchema>
export type ChannelStatusDetail = z.infer<typeof channelStatusDetailSchema>
export type PublicChannelStatusMonitor = z.infer<
  typeof publicChannelStatusMonitorSchema
>
export type PublicChannelStatusSummary = z.infer<
  typeof publicChannelStatusSummarySchema
>
