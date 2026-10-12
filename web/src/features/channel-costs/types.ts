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
export interface ChannelCostChannel {
  id: number
  name: string
  type: number
  status: number
  deleted: boolean
  /** The channel's own ratio; null means it uses the default ratio. */
  ratio: number | null
  today_quota: number
  yesterday_quota: number
  week_quota: number
}

export interface ChannelCostStats {
  default_ratio: number
  log_consume_enabled: boolean
  channels: ChannelCostChannel[]
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
