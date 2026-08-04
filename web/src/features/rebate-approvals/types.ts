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
export interface RebateApprovalRequest {
  id: number
  user_id: number
  username?: string
  display_name?: string
  invite_reward_quota: number
  recharge_rebate_quota: number
  total_quota: number
  status: 'pending' | 'approved' | 'rejected'
  created_at: number
  reviewed_at?: number
  reviewed_by?: number
  reject_reason?: string
}

export interface RebateApprovalListResponse {
  items: RebateApprovalRequest[]
  total: number
  page?: number
  page_size?: number
}

export interface RebateApproveAllResult {
  approved_count: number
}

export interface RebateApprovalRechargeSource {
  invited_user_id: number
  invited_display_name: string
  payment_provider: string
  payment_method: string
  credited_quota: number
  rebate_quota: number
  complete_time: number
}

export interface RebateApprovalInvitedUser {
  id: number
  username: string
  display_name: string
  created_at: number
  last_login_at: number
  is_deleted: boolean
}

export interface RebateApprovalDetail extends RebateApprovalRequest {
  invited_users: RebateApprovalInvitedUser[]
  invited_count: number
  total_invited_recharge_quota: number
  recharge_rebate_rate: number
  recharge_sources: RebateApprovalRechargeSource[]
}

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}
