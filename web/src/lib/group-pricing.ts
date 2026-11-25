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
const groupCategories = ['Claude', 'GPT', 'Other'] as const
type GroupCategory = (typeof groupCategories)[number]

const nameOrder = new Intl.Collator(undefined, { numeric: true })

function sortableRatio(value: number | string | null | undefined): number {
  if (value == null || (typeof value === 'string' && value.trim() === '')) {
    return Infinity
  }
  const ratio = Number(value)
  return Number.isFinite(ratio) ? ratio : Infinity
}

export function categorizePricingGroups<T>(
  items: readonly T[],
  getName: (item: T) => string,
  getRatio: (item: T) => number | string | null | undefined
): { label: GroupCategory; items: T[] }[] {
  const categories: Record<GroupCategory, T[]> = {
    Claude: [],
    GPT: [],
    Other: [],
  }
  for (const item of items) {
    const name = getName(item).toLowerCase()
    if (name.includes('claude')) {
      categories.Claude.push(item)
    } else if (name.includes('gpt') || name.includes('codex')) {
      categories.GPT.push(item)
    } else {
      categories.Other.push(item)
    }
  }
  return groupCategories
    .filter((label) => categories[label].length > 0)
    .map((label) => ({
      label,
      items: categories[label].sort(
        (a, b) =>
          sortableRatio(getRatio(a)) - sortableRatio(getRatio(b)) ||
          nameOrder.compare(getName(a), getName(b))
      ),
    }))
}
