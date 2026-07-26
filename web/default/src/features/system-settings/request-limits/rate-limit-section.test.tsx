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
import { expect, mock, test } from 'bun:test'

import { renderToStaticMarkup } from 'react-dom/server'

import type { RateLimitFormValues } from './types'

type MutationOptions = {
  onError?: (error: Error) => void
}

let mutationOptions: MutationOptions | undefined
let toastErrorMessage = ''

mock.module('@tanstack/react-query', () => ({
  useMutation: (options: MutationOptions) => {
    mutationOptions = options
    return {
      isPending: false,
      mutate: () => undefined,
    }
  },
  useQueryClient: () => ({
    invalidateQueries: () => Promise.resolve(),
  }),
}))

mock.module('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => `translated:${key}`,
  }),
}))

mock.module('sonner', () => ({
  toast: {
    error: (message: string) => {
      toastErrorMessage = message
    },
    success: () => undefined,
  },
}))

mock.module('./distillation-penalties-table', () => ({
  DistillationPenaltiesTable: () => null,
}))

mock.module('./distillation-settings', () => ({
  DistillationSettings: () => null,
}))

mock.module('./rate-limit-visual-editor', () => ({
  RateLimitVisualEditor: () => null,
}))

mock.module('./user-rate-limit-editor', () => ({
  UserRateLimitEditor: () => null,
}))

const { RateLimitSection } = await import('./rate-limit-section')

const defaultValues: RateLimitFormValues = {
  ModelRequestRateLimitEnabled: true,
  ModelRequestRateLimitDurationMinutes: 1,
  ModelRequestRateLimitCount: 100,
  ModelRequestRateLimitSuccessCount: 50,
  ModelRequestRateLimitGroup: '{}',
  ModelRequestRateLimitUser: '{}',
  ModelRequestRateLimitDistillationEnabled: false,
  ModelRequestRateLimitDistillationThreshold: 0,
  ModelRequestRateLimitDistillationRPM: 0,
  ModelRequestRateLimitDistillationPenaltyMinutes: 0,
  ModelRequestRateLimitDistillationObservationMinutes: 0,
}

test('translates a server-provided save error before displaying it', () => {
  toastErrorMessage = ''

  renderToStaticMarkup(<RateLimitSection defaultValues={defaultValues} />)
  mutationOptions?.onError?.(new Error('Failed to save rate limits'))

  expect(toastErrorMessage).toBe('translated:Failed to save rate limits')
})
