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
import { describe, expect, mock, test } from 'bun:test'

import { renderToStaticMarkup } from 'react-dom/server'
import { useForm } from 'react-hook-form'

import { Form } from '@/components/ui/form'

import type { RateLimitFormValues } from './types'

mock.module('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const { DistillationSettings } = await import('./distillation-settings')

const formValues: RateLimitFormValues = {
  ModelRequestRateLimitEnabled: true,
  ModelRequestRateLimitDurationMinutes: 1,
  ModelRequestRateLimitCount: 100,
  ModelRequestRateLimitSuccessCount: 80,
  ModelRequestRateLimitGroup: '{}',
  ModelRequestRateLimitUser: '{}',
  ModelRequestRateLimitDistillationEnabled: true,
  ModelRequestRateLimitDistillationThreshold: 60,
  ModelRequestRateLimitDistillationRPM: 10,
  ModelRequestRateLimitDistillationPenaltyMinutes: 30,
  ModelRequestRateLimitDistillationObservationMinutes: 1440,
}

function DistillationSettingsHarness(props: { enabled: boolean }) {
  const form = useForm<RateLimitFormValues>({
    defaultValues: {
      ...formValues,
      ModelRequestRateLimitDistillationEnabled: props.enabled,
    },
  })

  return (
    <Form {...form}>
      <DistillationSettings control={form.control} enabled={props.enabled} />
    </Form>
  )
}

describe('DistillationSettings', () => {
  test('keeps configured values visible but disables controls while off', () => {
    const html = renderToStaticMarkup(
      <DistillationSettingsHarness enabled={false} />
    )

    expect(html).toContain('Enable distillation detection')
    expect(html).toContain('Detection threshold')
    expect(html).toContain('Penalty RPM')
    expect(html).toContain('Penalty duration')
    expect(html).toContain('Observation period')
    expect(html).toContain('value="60"')
    expect(html).toContain('value="10"')
    expect(html).toContain('value="30"')
    expect(html).toContain('value="1440"')
    expect(html.match(/\sdisabled=""/g)?.length).toBe(4)
    expect(html).toContain('non-stream requests / minute')
    expect(html).toContain('requests / minute')
    expect(html).toContain('minutes')
  })

  test('enables numeric controls and uses a responsive four-field grid', () => {
    const html = renderToStaticMarkup(<DistillationSettingsHarness enabled />)

    expect(html).not.toContain('disabled=""')
    expect(html).toContain('sm:grid-cols-2')
    expect(html).toContain('xl:grid-cols-4')
  })
})
