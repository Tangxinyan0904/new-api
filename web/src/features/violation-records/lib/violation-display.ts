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
import { formatTimestamp } from '@/lib/format'

import type { DistillationViolationRecord } from '../types'

export function getViolationActionLabel(action: string): string {
  switch (action) {
    case 'temporary_limit':
      return 'Temporary limit'
    case 'permanent_ban':
      return 'Permanent non-stream ban'
    default:
      return 'Unknown'
  }
}

export function getViolationEffectiveLabel(
  record: DistillationViolationRecord
): string {
  if (record.action === 'permanent_ban') {
    return 'Permanent'
  }
  return formatTimestamp(record.effective_until)
}
