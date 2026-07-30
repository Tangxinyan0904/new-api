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
export function isExactIPAddress(value: string): boolean {
  const trimmed = value.trim()
  if (
    !trimmed ||
    trimmed.includes('/') ||
    trimmed.includes('[') ||
    trimmed.includes(']') ||
    /\s/.test(trimmed)
  ) {
    return false
  }

  if (!trimmed.includes(':')) {
    const parts = trimmed.split('.')
    return (
      parts.length === 4 &&
      parts.every((part) => {
        if (!/^\d+$/.test(part) || (part.length > 1 && part.startsWith('0'))) {
          return false
        }
        const segment = Number(part)
        return segment >= 0 && segment <= 255
      })
    )
  }

  const zoneIndex = trimmed.lastIndexOf('%')
  let candidate = trimmed
  if (zoneIndex >= 0) {
    if (
      zoneIndex === 0 ||
      zoneIndex === trimmed.length - 1 ||
      trimmed.indexOf('%') !== zoneIndex
    ) {
      return false
    }
    candidate = trimmed.slice(0, zoneIndex)
  }
  try {
    const parsed = new URL(`http://[${candidate}]/`)
    return parsed.hostname.startsWith('[') && parsed.hostname.endsWith(']')
  } catch {
    return false
  }
}
