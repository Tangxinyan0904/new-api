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

import * as ReactQuery from '@tanstack/react-query'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

import type { RateLimitFormValues } from '../types'

type MutationOptions = {
  onError?: (error: Error) => void
}

let mutationOptions: MutationOptions | undefined
let toastErrorMessage = ''

mock.module('@tanstack/react-query', () => ({
  ...ReactQuery,
  useMutation: (options: MutationOptions) => {
    mutationOptions = options
    return {
      isPending: false,
      mutate: () => undefined,
    }
  },
}))

mock.module('sonner', () => ({
  toast: {
    error: (message: string) => {
      toastErrorMessage = message
    },
    success: () => undefined,
  },
}))

mock.module('../distillation-penalties-table', () => ({
  DistillationPenaltiesTable: () => null,
}))

mock.module('../distillation-settings', () => ({
  DistillationSettings: () => null,
}))

mock.module('../rate-limit-visual-editor', () => ({
  RateLimitVisualEditor: () => null,
}))

mock.module('../user-rate-limit-editor', () => ({
  UserRateLimitEditor: () => null,
}))

const { RateLimitSection } = await import('../rate-limit-section')

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Failed to save rate limits': 'translated:Failed to save rate limits',
      },
    },
  },
})

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
  const queryClient = new ReactQuery.QueryClient()

  renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <ReactQuery.QueryClientProvider client={queryClient}>
        <RateLimitSection defaultValues={defaultValues} />
      </ReactQuery.QueryClientProvider>
    </I18nextProvider>
  )
  mutationOptions?.onError?.(new Error('Failed to save rate limits'))

  expect(toastErrorMessage).toBe('translated:Failed to save rate limits')
})
