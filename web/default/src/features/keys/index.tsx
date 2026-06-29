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
import { CopyButton } from '@/components/copy-button'
import { getBgColorClass } from '@/lib/colors'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { ApiKeysDialogs } from './components/api-keys-dialogs'
import { ApiKeysPrimaryButtons } from './components/api-keys-primary-buttons'
import { ApiKeysProvider } from './components/api-keys-provider'
import { ApiKeysTable } from './components/api-keys-table'

type ApiInfoAddress = {
  url: string
  route?: string
  description?: string
  color?: string
}

function appendApiV1Path(url: string): string {
  const trimmed = url.trim()
  if (!trimmed) return trimmed

  const withoutTrailingSlash = trimmed.replace(/\/+$/, '')
  if (withoutTrailingSlash.endsWith('/v1')) {
    return withoutTrailingSlash
  }

  return `${withoutTrailingSlash}/v1`
}

function isApiInfoAddress(value: unknown): value is ApiInfoAddress {
  if (typeof value !== 'object' || value === null) {
    return false
  }

  const candidate = value as Record<string, unknown>
  return typeof candidate.url === 'string' && candidate.url.trim().length > 0
}

function getApiInfoAddresses(value: unknown): ApiInfoAddress[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value.filter(isApiInfoAddress)
}

function ApiKeysApiAddresses() {
  const { t } = useTranslation()
  const { status, loading } = useStatus()
  const enabled = status ? status.api_info_enabled !== false : false
  const apiInfoAddresses = enabled ? getApiInfoAddresses(status?.api_info) : []

  if (loading || apiInfoAddresses.length === 0) {
    return null
  }

  return (
    <section
      aria-label={t('API Addresses')}
      className='border-border/70 bg-muted/20 rounded-lg border p-3'
    >
      <div className='mb-2 text-sm font-medium'>{t('API Addresses')}</div>
      <div className='grid gap-2'>
        {apiInfoAddresses.map((item) => {
          const title = item.route || item.description || t('API URL')
          const v1Url = appendApiV1Path(item.url)

          return (
            <div
              key={`${item.url}-${title}`}
              className='border-border/70 bg-background flex flex-col gap-2 rounded-md border px-3 py-2 sm:flex-row sm:items-center sm:justify-start sm:gap-3'
            >
              <div className='flex shrink-0 flex-wrap items-center gap-2'>
                <CopyButton
                  value={item.url}
                  variant='outline'
                  size='sm'
                  iconClassName='size-3.5'
                  tooltip={t('Copy URL')}
                  aria-label={t('Copy URL')}
                >
                  {t('Copy')}
                </CopyButton>
                <CopyButton
                  value={v1Url}
                  variant='outline'
                  size='sm'
                  iconClassName='size-3.5'
                  tooltip={t('Copy with /v1')}
                  aria-label={t('Copy with /v1')}
                >
                  {t('Copy with /v1')}
                </CopyButton>
              </div>

              <div className='flex min-w-0 flex-1 items-center gap-2'>
                <span
                  className={cn(
                    'size-2 shrink-0 rounded-full',
                    getBgColorClass(item.color)
                  )}
                  aria-hidden='true'
                />
                <div className='min-w-0'>
                  <div className='truncate text-sm font-medium'>{title}</div>
                  <div className='text-muted-foreground truncate font-mono text-xs'>
                    {item.url}
                  </div>
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </section>
  )
}

export function ApiKeys() {
  const { t } = useTranslation()
  return (
    <ApiKeysProvider>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('API Keys')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <ApiKeysPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <ApiKeysApiAddresses />
            <ApiKeysTable />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ApiKeysDialogs />
    </ApiKeysProvider>
  )
}
