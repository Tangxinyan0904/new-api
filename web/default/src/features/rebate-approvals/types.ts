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
