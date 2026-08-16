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

const providerSchema = z.enum(['openai', 'anthropic', 'gemini'])
const apiModeSchema = z.enum([
  'chat_completions',
  'responses',
  'messages',
  'generate_content',
])

const supportedModes = {
  openai: new Set(['chat_completions', 'responses']),
  anthropic: new Set(['messages']),
  gemini: new Set(['generate_content']),
} as const

const channelMonitorFormBaseSchema = z.object({
  internal_name: z
    .string()
    .trim()
    .min(1, 'Internal name is required')
    .max(128, 'Internal name is too long'),
  public_name: z
    .string()
    .trim()
    .min(1, 'Public name is required')
    .max(128, 'Public name is too long'),
  provider: providerSchema,
  api_mode: apiModeSchema,
  endpoint: z
    .string()
    .trim()
    .min(1, 'Endpoint is required')
    .max(2048, 'Endpoint is too long'),
  api_key: z.string().trim(),
  primary_model: z
    .string()
    .trim()
    .min(1, 'Primary model is required')
    .max(256, 'Primary model is too long'),
  extra_models: z.array(
    z.string().trim().min(1, 'Extra model cannot be empty').max(256)
  ),
  interval_seconds: z
    .number()
    .int()
    .min(60, 'Interval must be at least 60 seconds')
    .max(3600, 'Interval must not exceed 3600 seconds'),
  degraded_threshold_ms: z
    .number()
    .int()
    .positive('Degraded threshold must be positive'),
  timeout_seconds: z
    .number()
    .int()
    .min(1, 'Timeout must be at least 1 second')
    .max(120, 'Timeout must not exceed 120 seconds'),
  enabled: z.boolean(),
  public_enabled: z.boolean(),
})

export type ChannelMonitorFormValues = z.infer<
  typeof channelMonitorFormBaseSchema
>

export const CHANNEL_MONITOR_FORM_DEFAULTS: ChannelMonitorFormValues = {
  internal_name: '',
  public_name: '',
  provider: 'openai',
  api_mode: 'chat_completions',
  endpoint: '',
  api_key: '',
  primary_model: '',
  extra_models: [],
  interval_seconds: 60,
  degraded_threshold_ms: 6000,
  timeout_seconds: 30,
  enabled: true,
  public_enabled: false,
}

export function getChannelMonitorFormSchema(isEdit: boolean) {
  return channelMonitorFormBaseSchema.superRefine((value, context) => {
    if (!supportedModes[value.provider].has(value.api_mode)) {
      context.addIssue({
        code: 'custom',
        path: ['api_mode'],
        message: 'Unsupported provider and API mode combination',
      })
    }

    if (!isEdit && value.api_key.length === 0) {
      context.addIssue({
        code: 'custom',
        path: ['api_key'],
        message: 'Credential is required when creating a monitor',
      })
    }

    const modelNames = [value.primary_model, ...value.extra_models]
    if (modelNames.length > 20) {
      context.addIssue({
        code: 'custom',
        path: ['extra_models'],
        message: 'A monitor supports at most 20 models',
      })
    }
    if (new Set(modelNames).size !== modelNames.length) {
      context.addIssue({
        code: 'custom',
        path: ['extra_models'],
        message: 'Monitor model names must be unique',
      })
    }

    let endpoint: URL
    try {
      endpoint = new URL(value.endpoint)
    } catch {
      context.addIssue({
        code: 'custom',
        path: ['endpoint'],
        message: 'Endpoint must be a valid URL',
      })
      return
    }
    if (endpoint.protocol !== 'http:' && endpoint.protocol !== 'https:') {
      context.addIssue({
        code: 'custom',
        path: ['endpoint'],
        message: 'Endpoint must use HTTP or HTTPS',
      })
    }
    if (
      endpoint.username ||
      endpoint.password ||
      endpoint.search ||
      endpoint.hash
    ) {
      context.addIssue({
        code: 'custom',
        path: ['endpoint'],
        message: 'Endpoint must be a base URL without credentials or query',
      })
    }
    const path = endpoint.pathname.replace(/\/+$/, '').toLowerCase()
    const protocolPaths = [
      '/v1/chat/completions',
      '/chat/completions',
      '/v1/responses',
      '/responses',
      '/v1/messages',
      '/messages',
    ]
    if (
      protocolPaths.some((suffix) => path.endsWith(suffix)) ||
      path.endsWith(':generatecontent')
    ) {
      context.addIssue({
        code: 'custom',
        path: ['endpoint'],
        message: 'Endpoint must not include the protocol request path',
      })
    }
  })
}
