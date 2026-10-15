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

export const channelStatusSchema = z.enum([
  'operational',
  'degraded',
  'failed',
  'unknown',
])

export const channelStatusCountsSchema = z.object({
  operational: z.number().int().nonnegative(),
  degraded: z.number().int().nonnegative(),
  failed: z.number().int().nonnegative(),
  unknown: z.number().int().nonnegative(),
})

/** Request outcomes of a group, a model or a time bucket in the last hour. */
export const channelStatusFiguresSchema = z.object({
  status: channelStatusSchema,
  /** Percentage of requests that succeeded; null without requests. */
  success_rate: z.number().finite().nullable(),
  success_count: z.number().int().nonnegative(),
  failure_count: z.number().int().nonnegative(),
  /** Mean duration of the successful requests. */
  average_duration_ms: z.number().finite().nonnegative().nullable(),
})

export const channelStatusTimelinePointSchema =
  channelStatusFiguresSchema.extend({
    start_at: z.number().int().nonnegative(),
  })

export const channelStatusItemSchema = channelStatusFiguresSchema.extend({
  id: z.number().int().positive(),
  public_name: z.string(),
  name_color: z.string(),
  /** Only sent to staff. */
  group: z.string().optional(),
  public_enabled: z.boolean(),
  timeline: z.array(channelStatusTimelinePointSchema),
})

export const channelStatusResponseSchema = z.object({
  last_updated_at: z.number().int().nonnegative(),
  window_start: z.number().int().nonnegative(),
  bucket_seconds: z.number().int().positive(),
  counts: channelStatusCountsSchema,
  items: z.array(channelStatusItemSchema),
})

export const channelModelStatusSchema = channelStatusFiguresSchema.extend({
  model: z.string(),
})

export const channelStatusDetailSchema = z.object({
  last_updated_at: z.number().int().nonnegative(),
  monitor: channelStatusItemSchema,
  models: z.array(channelModelStatusSchema),
})

export const publicChannelStatusMonitorSchema = z.object({
  id: z.number().int().positive(),
  public_name: z.string(),
  name_color: z.string(),
  status: channelStatusSchema,
  success_rate: z.number().finite().nullable(),
})

export const publicChannelStatusSummarySchema = z.object({
  last_updated_at: z.number().int().nonnegative(),
  counts: channelStatusCountsSchema,
  monitors: z.array(publicChannelStatusMonitorSchema),
})

export type ChannelStatus = z.infer<typeof channelStatusSchema>
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

/** What staff enter to show a group on the status page. */
export interface ChannelStatusMonitorInput {
  group: string
  public_name: string
  name_color: string
  public_enabled: boolean
}
