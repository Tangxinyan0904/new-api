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
export type RegistrationIPAccount = {
  user_id: number
  username: string
  display_name: string
  status: number
  user_created_at: number
  registration_at: number
  deleted: boolean
  restore_eligible: boolean
  auto_disabled_at: number
}

export type BlockedRegistrationIP = {
  ip: string
  current_cycle: number
  registration_count: number
  blocked_at: number
  associated_account_count: number
  accounts: RegistrationIPAccount[]
}

export type RegistrationIPAllowlistItem = {
  ip: string
  current_cycle: number
  registration_count: number
  created_at: number
  updated_at: number
}

export type RegistrationIPMutationResult = {
  ip: string
  affected_user_ids: number[]
  affected_account_count: number
  allowlisted: boolean
}

export type PageData<T> = {
  page: number
  page_size: number
  total: number
  items: T[]
}

export type RegistrationIPApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}
