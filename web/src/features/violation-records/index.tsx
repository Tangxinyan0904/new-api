import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { ViolationRecordsTable } from './components/violation-records-table'

export function ViolationRecords() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Violation Records')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <ViolationRecordsTable />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
