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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { useState } from 'react'

import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { canvasStatusQueryKey, getCanvasStatus, startCanvasUpdate } from './api'
import type { CanvasStatus, CanvasUpdateTask } from './types'

/** How often the canvas versions are read again while nothing is running. */
export const CANVAS_STATUS_INTERVAL = 30 * 60 * 1000
/** How often a running update is followed. */
export const CANVAS_UPDATE_POLL_INTERVAL = 3000

export function isCanvasTaskActive(task: CanvasUpdateTask | null | undefined) {
  return task?.status === 'pending' || task?.status === 'running'
}

/** The canvas versions and update action, for the root user only. */
export function useCanvasUpdate() {
  const queryClient = useQueryClient()
  const isRoot = useAuthStore(
    (state) => (state.auth.user?.role ?? 0) >= ROLE.SUPER_ADMIN
  )
  const [checking, setChecking] = useState(false)
  const [starting, setStarting] = useState(false)
  const query = useQuery({
    queryKey: canvasStatusQueryKey,
    queryFn: () => getCanvasStatus(),
    enabled: isRoot,
    staleTime: CANVAS_STATUS_INTERVAL,
    refetchInterval: (current) =>
      isCanvasTaskActive(current.state.data?.task)
        ? CANVAS_UPDATE_POLL_INTERVAL
        : CANVAS_STATUS_INTERVAL,
    refetchIntervalInBackground: false,
    retry: false,
    meta: { errorToast: false },
  })
  const status = query.data

  /** Shows a queued or running task at once, before the next status read. */
  const followTask = (task: CanvasUpdateTask) => {
    queryClient.setQueryData<CanvasStatus>(
      canvasStatusQueryKey,
      (old) =>
        old && {
          ...old,
          task,
          current_version: '',
          update_available: false,
          check_error: null,
        }
    )
  }

  const checkNow = async () => {
    setChecking(true)
    try {
      const fresh = await getCanvasStatus(true)
      const current =
        queryClient.getQueryData<CanvasStatus>(canvasStatusQueryKey)
      // An answer read before an update was queued must not hide it.
      if (
        isCanvasTaskActive(current?.task) &&
        !isCanvasTaskActive(fresh.task)
      ) {
        await queryClient.invalidateQueries({ queryKey: canvasStatusQueryKey })
      } else {
        queryClient.setQueryData(canvasStatusQueryKey, fresh)
      }
    } catch (error) {
      handleServerError(error)
    } finally {
      setChecking(false)
    }
  }

  /** Resolves true when the server queued the update. */
  const startUpdate = async (): Promise<boolean> => {
    setStarting(true)
    try {
      const result = await startCanvasUpdate()
      if (!result.success || !result.data) {
        handleServerError(result)
        return false
      }
      followTask(result.data)
      return true
    } catch (error) {
      // 409: another update is already running; follow that one.
      const running: unknown = isAxiosError(error)
        ? error.response?.data?.data
        : undefined
      if (running && typeof running === 'object' && 'task_id' in running) {
        followTask(running as CanvasUpdateTask)
      }
      handleServerError(error)
      return false
    } finally {
      await queryClient.invalidateQueries({ queryKey: canvasStatusQueryKey })
      setStarting(false)
    }
  }

  return {
    isRoot,
    status,
    loading: query.isLoading,
    checking,
    starting,
    updating: starting || isCanvasTaskActive(status?.task),
    checkNow,
    startUpdate,
  }
}
