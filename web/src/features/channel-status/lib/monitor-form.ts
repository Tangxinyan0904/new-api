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

export const NAME_COLOR_PATTERN = /^#[0-9A-Fa-f]{6}$/

export const monitorFormSchema = z.object({
  group: z.string().trim().min(1, 'Choose a group'),
  public_name: z
    .string()
    .trim()
    .min(1, 'Display name is required')
    .max(128, 'Display name is too long'),
  name_color: z
    .string()
    .trim()
    .refine(
      (value) => value === '' || NAME_COLOR_PATTERN.test(value),
      'Name color must use #RRGGBB format'
    )
    .transform((value) => value.toUpperCase()),
  public_enabled: z.boolean(),
})

export type MonitorFormInput = z.input<typeof monitorFormSchema>
export type MonitorFormValues = z.output<typeof monitorFormSchema>
