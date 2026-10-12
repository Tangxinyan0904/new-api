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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { CreateTicketDialog } from './components/create-ticket-dialog'
import { StaffTicketsTable } from './components/staff-tickets-table'
import { UserTicketsTable } from './components/user-tickets-table'

export { StaffTicketDetail } from './components/staff-ticket-detail'
export { UserTicketDetail } from './components/user-ticket-detail'

export function Tickets() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('My Tickets')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <CreateTicketDialog />
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <UserTicketsTable />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

export function TicketManagement() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Ticket Management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <StaffTicketsTable />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
