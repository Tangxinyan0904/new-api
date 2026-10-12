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
import { TICKET_IMAGE_TYPES, TICKET_LIMITS } from '../constants'

export interface TicketImageSelection {
  accepted: File[]
  /** i18n key of the first rejection reason, with its params. */
  error?: { key: string; params?: Record<string, number> }
}

/**
 * Adds picked files to the current selection, keeping only supported images
 * within the size and count limits. The server checks the file content again.
 */
export function addTicketImages(
  current: File[],
  picked: File[]
): TicketImageSelection {
  const accepted = [...current]
  let error: TicketImageSelection['error']
  for (const file of picked) {
    if (!TICKET_IMAGE_TYPES.includes(file.type)) {
      error ??= { key: 'Only PNG, JPEG, and WebP images are supported' }
      continue
    }
    if (file.size > TICKET_LIMITS.MAX_IMAGE_BYTES) {
      error ??= {
        key: 'Each image must be {{max}} MB or smaller',
        params: { max: TICKET_LIMITS.MAX_IMAGE_BYTES / 1024 / 1024 },
      }
      continue
    }
    if (accepted.length >= TICKET_LIMITS.MAX_IMAGES) {
      error ??= {
        key: 'You can attach at most {{max}} images',
        params: { max: TICKET_LIMITS.MAX_IMAGES },
      }
      continue
    }
    accepted.push(file)
  }
  return { accepted, error }
}

/** Image files from a paste event, so screenshots can be pasted directly. */
export function pastedImages(event: { clipboardData: DataTransfer }): File[] {
  return [...event.clipboardData.files].filter((file) =>
    file.type.startsWith('image/')
  )
}
