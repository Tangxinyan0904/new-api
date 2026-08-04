export type DistillationViolationAction = 'temporary_limit' | 'permanent_ban'

export interface DistillationViolationRecord {
  id: number
  cycle_started_at: number
  triggered_at: number
  request_count: number
  detection_threshold: number
  penalty_rpm: number
  action: DistillationViolationAction | string
  effective_until: number
  created_at: number
}

export interface ViolationRecordsPage {
  items: DistillationViolationRecord[]
  total: number
  page?: number
  page_size?: number
}

export interface ViolationRecordsApiResponse {
  success?: boolean
  message?: string
  data?: ViolationRecordsPage
}
