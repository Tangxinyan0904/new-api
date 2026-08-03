import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { RebateApprovalsTable } from './components/rebate-approvals-table'

export function RebateApprovals() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Rebate Approvals')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <RebateApprovalsTable />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
