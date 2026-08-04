import { api } from '@/lib/api'

import type { ViolationRecordsApiResponse } from './types'

export async function getViolationRecords(
  page: number,
  pageSize: number
): Promise<ViolationRecordsApiResponse> {
  const search = new URLSearchParams({
    p: String(page),
    page_size: String(pageSize),
  })
  const response = await api.get(
    `/api/user/distillation/violations/self?${search.toString()}`
  )
  return response.data
}
