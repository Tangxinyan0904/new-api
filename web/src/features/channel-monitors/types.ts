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

export const channelMonitorProviderSchema = z.enum([
  'openai',
  'anthropic',
  'gemini',
])

export const channelMonitorApiModeSchema = z.enum([
  'chat_completions',
  'responses',
  'messages',
  'generate_content',
])

export const channelMonitorStatusSchema = z.enum([
  'operational',
  'degraded',
  'failed',
  'unknown',
])

export const channelMonitorErrorCategorySchema = z.enum([
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

export const channelMonitorSchema = z.object({
  id: z.number().int().positive(),
  internal_name: z.string(),
  public_name: z.string(),
  name_color: z.string(),
  provider: channelMonitorProviderSchema,
  api_mode: channelMonitorApiModeSchema,
  endpoint: z.string(),
  credential_configured: z.boolean(),
  credential_masked: z.string(),
  primary_model: z.string(),
  extra_models: z.array(z.string()),
  interval_seconds: z.number().int(),
  degraded_threshold_ms: z.number().int(),
  timeout_seconds: z.number().int(),
  enabled: z.boolean(),
  public_enabled: z.boolean(),
  last_checked_at: z.number().int(),
  next_check_at: z.number().int(),
  schedule_blocked_reason: z.string(),
  latest_status: channelMonitorStatusSchema,
  latest_latency_ms: z.number().int().nullable(),
  latest_error_category: channelMonitorErrorCategorySchema,
  lease_owner: z.string(),
  lease_until: z.number().int(),
  created_by: z.number().int(),
  created_at: z.number().int(),
  updated_at: z.number().int(),
})

export const channelMonitorCheckSchema = z.object({
  id: z.number().int(),
  monitor_id: z.number().int().positive(),
  model: z.string(),
  status: channelMonitorStatusSchema.exclude(['unknown']),
  latency_ms: z.number().int(),
  error_category: channelMonitorErrorCategorySchema,
  http_status: z.number().int(),
  error_message: z.string(),
  checked_at: z.number().int(),
})

export type ChannelMonitorProvider = z.infer<
  typeof channelMonitorProviderSchema
>
export type ChannelMonitorApiMode = z.infer<typeof channelMonitorApiModeSchema>
export type ChannelMonitorStatus = z.infer<typeof channelMonitorStatusSchema>
export type ChannelMonitorErrorCategory = z.infer<
  typeof channelMonitorErrorCategorySchema
>
export type ChannelMonitor = z.infer<typeof channelMonitorSchema>
export type ChannelMonitorCheck = z.infer<typeof channelMonitorCheckSchema>

export type ChannelMonitorPayload = {
  internal_name: string
  public_name: string
  name_color: string
  provider: ChannelMonitorProvider
  api_mode: ChannelMonitorApiMode
  endpoint: string
  api_key: string
  primary_model: string
  extra_models: string[]
  interval_seconds: number
  degraded_threshold_ms: number
  timeout_seconds: number
  enabled: boolean
  public_enabled: boolean
}

export type ChannelMonitorListParams = {
  p: number
  page_size: number
  search?: string
  provider?: ChannelMonitorProvider
  enabled?: boolean
  public_enabled?: boolean
}

export type ChannelMonitorHistoryParams = {
  p: number
  page_size: number
  model?: string
  status?: ChannelMonitorStatus
  error_category?: Exclude<ChannelMonitorErrorCategory, ''>
  start_time?: number
  end_time?: number
}

export type ChannelMonitorPage = {
  items: ChannelMonitor[]
  total: number
  page: number
  page_size: number
}

export type ChannelMonitorHistoryPage = {
  items: ChannelMonitorCheck[]
  total: number
  page: number
  page_size: number
}
