import { SectionPageLayout } from '@/components/layout'
import { RebateApprovalsTable } from './components/rebate-approvals-table'

export function RebateApprovals() {
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>返利审批</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <RebateApprovalsTable />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}