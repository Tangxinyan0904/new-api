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

/** Matches maxUpstreamUsdRate in controller/channel_cost.go. */
export const MAX_UPSTREAM_USD_RATE = 100

export const upstreamBillingFormSchema = z
  .object({
    kind: z.enum(['none', 'sub2api', 'newapi']),
    usd_rate: z.number(),
    upstream_user_id: z.number(),
    access_token: z.string().trim(),
    access_token_set: z.boolean(),
  })
  .superRefine((values, ctx) => {
    if (values.kind === 'none') return
    if (
      !Number.isFinite(values.usd_rate) ||
      values.usd_rate <= 0 ||
      values.usd_rate > MAX_UPSTREAM_USD_RATE
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['usd_rate'],
        message: 'The dollar rate must be greater than 0 and at most 100',
      })
    }
    if (values.kind !== 'newapi') return
    if (
      !Number.isInteger(values.upstream_user_id) ||
      values.upstream_user_id <= 0
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['upstream_user_id'],
        message: 'Enter your user ID on the upstream',
      })
    }
    if (values.access_token === '' && !values.access_token_set) {
      ctx.addIssue({
        code: 'custom',
        path: ['access_token'],
        message: 'Enter the system access token of your upstream account',
      })
    }
  })

export type UpstreamBillingFormValues = z.infer<
  typeof upstreamBillingFormSchema
>
