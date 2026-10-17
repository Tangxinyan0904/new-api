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
  /** Set when the cost is read from the upstream's bill instead. */
  upstream_billing: ChannelUpstreamBilling | null
  /** Set when a new-api login is saved for the channel's upstream address. */
  newapi_login: ChannelNewAPILogin | null
  today: ChannelCostUsage
  yesterday: ChannelCostUsage
  week: ChannelCostUsage
}

export type UpstreamBillingKind = 'sub2api' | 'newapi'

export interface ChannelUpstreamBilling {
  kind: UpstreamBillingKind
  /** How many of this site's dollars one upstream dollar is worth. */
  usd_rate: number
}

/**
 * The new-api login saved for an upstream address, shared by every new-api
 * channel on it. The access token is never sent back.
 */
export interface ChannelNewAPILogin {
  upstream_user_id: number
  /** The other channels on the address that read their bill with it. */
  shared_with: { id: number; name: string }[]
}

/** An upstream charge in the upstream's dollars and in this site's quota units. */
export interface UpstreamCostAmount {
  usd: number
  quota: number
}

export interface ChannelUpstreamCost {
  channel_id: number
  kind: UpstreamBillingKind
  /** Why the upstream could not be read; the amounts are then zero. */
  error: { key: string; params?: Record<string, unknown> } | null
  fetched_at: number
  today: UpstreamCostAmount
  yesterday: UpstreamCostAmount
  week: UpstreamCostAmount
}

/** How long an upstream account's balance lasts at its recent spending. */
export interface ChannelUpstreamAccount {
  kind: UpstreamBillingKind
  base_url: string
  channel_ids: number[]
  /** Balance and daily spend are in the upstream's dollars. */
  balance: number
  daily_spend: number
  /** Null when nothing was spent recently. */
  days_left: number | null
  /** The balance is used up or lasts less than a day. */
  low: boolean
}

export interface ChannelUpstreamReport {
  channels: ChannelUpstreamCost[]
  accounts: ChannelUpstreamAccount[]
}

export interface ChannelUpstreamBillingInput {
  /** Empty goes back to the cost ratio. */
  kind: UpstreamBillingKind | ''
  usd_rate: number
  upstream_user_id: number
  /** Empty uses the token saved for the channel's upstream address. */
  access_token: string
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
