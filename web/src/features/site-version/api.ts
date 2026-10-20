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
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type { ApiResponse, CanvasStatus, CanvasUpdateTask } from './types'

export const canvasStatusQueryKey = ['canvas-update', 'status'] as const

/** refresh asks the updater again instead of using its last minute's answer. */
export async function getCanvasStatus(refresh = false): Promise<CanvasStatus> {
  const res = await api.get('/api/canvas/status', {
    params: { refresh: refresh || undefined },
  })
  return requireServerSuccess(res.data as ApiResponse<CanvasStatus>)
    .data as CanvasStatus
}

/** Queues an update; the status reports it as it runs. */
export async function startCanvasUpdate(): Promise<
  ApiResponse<CanvasUpdateTask>
> {
  const res = await api.post('/api/canvas/update', null, {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  return res.data
}
