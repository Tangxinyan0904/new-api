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
import type { CellContext } from '@tanstack/react-table'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'
import { expect, test } from 'vitest'

import type { UsageLog } from '../../../data/schema'

const browserGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'customElements',
  'matchMedia',
] as const
const browserGlobalDescriptors = new Map(
  browserGlobals.map((key) => [
    key,
    Object.getOwnPropertyDescriptor(globalThis, key),
  ])
)
for (const key of browserGlobals) {
  if (key === 'navigator') continue
  Reflect.deleteProperty(globalThis, key)
}
Object.defineProperty(globalThis, 'navigator', {
  configurable: true,
  value: { userAgent: '' },
})

const { useCommonLogsColumns } = await import('../common-logs-columns')

for (const key of browserGlobals) {
  const descriptor = browserGlobalDescriptors.get(key)
  if (descriptor) Object.defineProperty(globalThis, key, descriptor)
}

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Stream: 'Stream',
        'Non-stream': 'Non-stream',
      },
    },
  },
})

const log: UsageLog = {
  id: 1,
  user_id: 1,
  created_at: 1,
  type: 2,
  content: '',
  username: 'user',
  token_name: 'key',
  model_name: 'model',
  quota: 1,
  prompt_tokens: 10,
  completion_tokens: 90,
  use_time: 2,
  is_stream: true,
  channel: 1,
  channel_name: 'channel',
  token_id: 1,
  group: 'default',
  ip: '127.0.0.1',
  other: '{}',
  request_id: 'request',
  upstream_request_id: '',
}

function StreamColumnProbe() {
  const columns = useCommonLogsColumns(false, false)
  const streamColumn = columns.find(
    (column) => 'accessorKey' in column && column.accessorKey === 'is_stream'
  )
  if (!streamColumn || typeof streamColumn.cell !== 'function') {
    throw new Error('Stream column is unavailable')
  }

  const cell = streamColumn.cell({
    row: {
      original: log,
      getValue: (key: string) => log[key as keyof UsageLog],
    },
  } as unknown as CellContext<UsageLog, unknown>)

  return <div data-column-size={streamColumn.size}>{cell}</div>
}

test('desktop stream column shows derived throughput in a compact width', () => {
  const html = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <StreamColumnProbe />
    </I18nextProvider>
  )

  expect(html).toContain('data-column-size="72"')
  expect(html).toContain('Stream')
  expect(html).toContain('45 t/s')
})
