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
import { expect, test } from 'bun:test'

import type { DistillationViolationRecord } from '../../types'
import {
  getViolationActionLabel,
  getViolationEffectiveLabel,
} from '../violation-display'

const permanentRecord: DistillationViolationRecord = {
  id: 2,
  cycle_started_at: 1_000,
  triggered_at: 1_700,
  request_count: 200,
  detection_threshold: 200,
  penalty_rpm: 10,
  action: 'permanent_ban',
  effective_until: 0,
  created_at: 1_700,
}

test('maps violation actions to stable display labels', () => {
  expect(getViolationActionLabel('temporary_limit')).toBe('Temporary limit')
  expect(getViolationActionLabel('permanent_ban')).toBe(
    'Permanent non-stream ban'
  )
  expect(getViolationActionLabel('future_action')).toBe('Unknown')
})

test('shows permanent instead of an empty effective timestamp', () => {
  expect(getViolationEffectiveLabel(permanentRecord)).toBe('Permanent')
})
