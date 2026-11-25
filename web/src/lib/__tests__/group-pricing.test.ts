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
import { describe, expect, it } from 'vitest'

import { categorizePricingGroups } from '../group-pricing'

describe('categorized group prices', () => {
  it('matches real names without case sensitivity and sorts by effective multiplier, then name', () => {
    const groups = [
      { name: 'CoDeX-10', ratio: 1 },
      { name: 'fallback', ratio: undefined },
      { name: 'gPt-regular', ratio: 2 },
      { name: 'free', ratio: 0 },
      { name: 'CODEX-2', ratio: 1 },
      { name: 'GPT-vip', ratio: 0.5 },
      { name: 'ClAuDe-gpt', ratio: 4 },
    ]
    const categories = categorizePricingGroups(
      groups,
      (g) => g.name,
      (g) => g.ratio
    )
    expect(
      categories.map(({ label, items }) => [label, items.map((g) => g.name)])
    ).toEqual([
      ['Claude', ['ClAuDe-gpt']],
      ['GPT', ['GPT-vip', 'CODEX-2', 'CoDeX-10', 'gPt-regular']],
      ['Other', ['free', 'fallback']],
    ])
  })

  it('omits categories without matching groups', () => {
    expect(
      categorizePricingGroups(
        ['custom'],
        (g) => g,
        () => 1
      )
    ).toEqual([{ label: 'Other', items: ['custom'] }])
  })
})
