import { formatTimestamp } from '@/lib/format'

import type { DistillationViolationRecord } from '../types'

export function getViolationActionLabel(action: string): string {
  switch (action) {
    case 'temporary_limit':
      return 'Temporary limit'
    case 'permanent_ban':
      return 'Permanent ban'
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
