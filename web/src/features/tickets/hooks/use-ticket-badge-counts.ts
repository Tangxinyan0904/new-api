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
import { useQuery } from '@tanstack/react-query'

import { hasPermission } from '@/lib/admin-permissions'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getStaffTicketSummary, getUserUnreadTicketCount } from '../api'
import { TICKET_BADGE_REFETCH_MS, ticketQueryKeys } from '../constants'

/**
 * Sidebar badge counts: tickets with unread support replies for the user, and
 * tickets awaiting support for staff who can view tickets.
 */
export function useTicketBadgeCounts() {
  const user = useAuthStore((state) => state.auth.user)
  const isStaff =
    !!user && user.role >= ROLE.ADMIN && hasPermission(user, 'ticket', 'read')
  const unread = useQuery({
    queryKey: ticketQueryKeys.userUnread,
    queryFn: getUserUnreadTicketCount,
    enabled: !!user,
    refetchInterval: TICKET_BADGE_REFETCH_MS,
    retry: false,
    meta: { errorToast: false },
  })
  const awaiting = useQuery({
    queryKey: ticketQueryKeys.staffSummary,
    queryFn: getStaffTicketSummary,
    enabled: isStaff,
    refetchInterval: TICKET_BADGE_REFETCH_MS,
    retry: false,
    meta: { errorToast: false },
  })
  return { unread: unread.data ?? 0, awaiting: awaiting.data ?? 0 }
}
