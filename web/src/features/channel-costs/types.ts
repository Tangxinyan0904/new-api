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
/** Usage in quota units: charged, and at the original price (before the group ratio). */
export interface ChannelCostUsage {
  quota: number
  base_quota: number
}

export interface ChannelCostChannel {
  id: number
  name: string
  type: number
  status: number
  deleted: boolean
  /** Cost ratio shared by all the channel's groups; null when none was entered. */
  ratio: number | null
  today: ChannelCostUsage
  yesterday: ChannelCostUsage
  week: ChannelCostUsage
}

export interface ChannelCostGroup {
  name: string
  /** The group's configured ratio; null when it has none. */
  ratio: number | null
  channels: ChannelCostChannel[]
}

export interface ChannelCostStats {
  log_consume_enabled: boolean
  groups: ChannelCostGroup[]
}

export type ChannelCostRange = 'today' | 'yesterday' | 'week'

/** Used amount, cost and profit of one range, all in quota units. */
export interface ChannelCostFigures {
  used: number
  cost: number
  profit: number
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  message_key?: string
  data?: T
}
