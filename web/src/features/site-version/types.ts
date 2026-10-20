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
/** A console message the server sends untranslated, as a key and parameters. */
export interface ServerMessage {
  key: string
  params?: Record<string, unknown>
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  message_key?: string
  data?: T
}

/** The newest canvas release on GitHub. */
export interface CanvasRelease {
  version: string
  published_at: string
  url: string
  notes: string
}

export type CanvasTaskStatus = 'pending' | 'running' | 'succeeded' | 'failed'

export interface CanvasUpdateResult {
  from_version: string
  to_version: string
  /** Containers the updater replaced; 0 means the canvas was already current. */
  updated: number
  error?: ServerMessage | null
}

export interface CanvasUpdateTask {
  task_id: string
  status: CanvasTaskStatus
  result: CanvasUpdateResult | null
  error: string
  created_at: number
  updated_at: number
}

export interface CanvasStatus {
  /** False until CANVAS_UPDATER_URL and CANVAS_UPDATER_TOKEN are set. */
  configured: boolean
  container: string
  /** Empty while an update is replacing the container. */
  current_version: string
  update_available: boolean
  checked_at: number
  check_error: ServerMessage | null
  release: CanvasRelease | null
  release_error: ServerMessage | null
  /** The latest update, running or finished. */
  task: CanvasUpdateTask | null
}
