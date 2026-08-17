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
import { afterAll, beforeEach, expect, mock, test } from 'bun:test'

import { api } from '@/lib/api'

import type { AddChannelRequest } from '../types'

const get = mock(async (..._args: unknown[]) => ({ data: { success: true } }))
const post = mock(async (..._args: unknown[]) => ({ data: { success: true } }))
const originalGet = api.get
const originalPost = api.post

const { createChannel, getChannels } = await import('../api')

beforeEach(() => {
  get.mockReset()
  post.mockReset()
  get.mockResolvedValue({ data: { success: true } })
  post.mockResolvedValue({ data: { success: true } })
  api.get = get as unknown as typeof api.get
  api.post = post as unknown as typeof api.post
})

afterAll(() => {
  api.get = originalGet
  api.post = originalPost
})

test('loads the channel list through the registered collection route', async () => {
  await getChannels({ group: 'vip' })

  expect(get.mock.calls[0]?.[0]).toBe('/api/channel/')
})

test('creates channels through the registered collection route', async () => {
  const payload: AddChannelRequest = {
    mode: 'single',
    channel: { name: 'Primary channel' },
  }

  await createChannel(payload)

  expect(post.mock.calls[0]?.[0]).toBe('/api/channel/')
})
