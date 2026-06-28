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
import { useState, useCallback } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Check, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { BadgeCell } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { updateApiKey } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import type { ApiKey, ApiKeyFormData } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { useApiKeys } from './api-keys-provider'

function CopyButtonContent({
  isLoading,
  isCopied,
  t,
}: {
  isLoading: boolean
  isCopied: boolean
  t: (key: string) => string
}) {
  if (isLoading) {
    return (
      <>
        <Loader2 className='size-3.5 animate-spin' />
        {t('Loading...')}
      </>
    )
  }

  if (isCopied) {
    return (
      <>
        <Check className='size-3.5 text-green-600' />
        {t('Copied!')}
      </>
    )
  }

  return t('Copy')
}

function getCopyTooltipLabel({
  isLoading,
  isCopied,
  t,
}: {
  isLoading: boolean
  isCopied: boolean
  t: (key: string) => string
}) {
  if (isLoading) return t('Loading...')
  if (isCopied) return t('Copied!')
  return t('Copy API key')
}

function getGroupUpdatePayload(
  apiKey: ApiKey,
  group: string
): ApiKeyFormData & { id: number } {
  return {
    id: apiKey.id,
    name: apiKey.name,
    remain_quota: apiKey.remain_quota,
    expired_time: apiKey.expired_time,
    unlimited_quota: apiKey.unlimited_quota,
    model_limits_enabled: apiKey.model_limits_enabled,
    model_limits: apiKey.model_limits || '',
    allow_ips: apiKey.allow_ips || '',
    group,
    cross_group_retry: group === 'auto' ? !!apiKey.cross_group_retry : false,
  }
}

function ensureCurrentGroupOption(
  options: ApiKeyGroupOption[],
  apiKey: ApiKey,
  fallbackLabel: string
): ApiKeyGroupOption[] {
  const currentGroup = apiKey.group || ''
  if (options.some((option) => option.value === currentGroup)) return options

  return [
    {
      value: currentGroup,
      label: currentGroup || fallbackLabel,
      desc: currentGroup || fallbackLabel,
    },
    ...options,
  ]
}

export function ApiKeyCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const {
    resolveRealKey,
    resolvedKeys,
    loadingKeys,
    copiedKeyId,
    markKeyCopied,
  } = useApiKeys()
  const [popoverOpen, setPopoverOpen] = useState(false)

  const isLoading = !!loadingKeys[apiKey.id]
  const resolvedFullKey = resolvedKeys[apiKey.id]
  const isCopied = copiedKeyId === apiKey.id
  const maskedKey = `sk-${apiKey.key}`

  const handlePopoverOpen = useCallback(
    (open: boolean) => {
      setPopoverOpen(open)
      if (open && !resolvedFullKey) {
        resolveRealKey(apiKey.id)
      }
    },
    [resolvedFullKey, resolveRealKey, apiKey.id]
  )

  const handleCopy = useCallback(async () => {
    const realKey = resolvedFullKey
    if (!realKey) {
      void resolveRealKey(apiKey.id)
      toast.info(t('API key is loading, please try again in a moment'))
      return
    }
    if (realKey) {
      const ok = await copyToClipboard(realKey)
      if (ok) markKeyCopied(apiKey.id)
    }
  }, [resolvedFullKey, resolveRealKey, apiKey.id, markKeyCopied, t])

  return (
    <div className='flex max-w-full min-w-0 items-center'>
      <Popover open={popoverOpen} onOpenChange={handlePopoverOpen}>
        <PopoverTrigger
          render={
            <Button
              variant='ghost'
              size='sm'
              className='text-muted-foreground h-7 max-w-full min-w-0 justify-start truncate px-0 font-mono text-xs hover:bg-transparent aria-expanded:bg-transparent'
            />
          }
        >
          <span className='truncate'>{maskedKey}</span>
        </PopoverTrigger>
        <PopoverContent
          className='w-auto max-w-[min(90vw,28rem)]'
          align='start'
        >
          <div className='space-y-2'>
            <p className='text-muted-foreground text-xs'>{t('Full API Key')}</p>
            {isLoading ? (
              <div className='flex items-center gap-2 py-2'>
                <Loader2 className='size-3.5 animate-spin' />
                <span className='text-muted-foreground text-xs'>
                  {t('Loading...')}
                </span>
              </div>
            ) : (
              <input
                readOnly
                value={resolvedFullKey || maskedKey}
                autoFocus
                onFocus={(e) => e.target.select()}
                className='bg-muted/50 w-full min-w-[280px] rounded-md border px-3 py-2 font-mono text-xs outline-none'
              />
            )}
          </div>
        </PopoverContent>
      </Popover>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='outline'
              size='sm'
              className='h-7 shrink-0 px-2 text-xs'
              onClick={handleCopy}
              onFocus={() => {
                if (!resolvedFullKey) void resolveRealKey(apiKey.id)
              }}
              onPointerEnter={() => {
                if (!resolvedFullKey) void resolveRealKey(apiKey.id)
              }}
              disabled={isLoading}
            />
          }
        >
          <CopyButtonContent
            isLoading={isLoading}
            isCopied={isCopied}
            t={t}
          />
        </TooltipTrigger>
        <TooltipContent>
          {getCopyTooltipLabel({ isLoading, isCopied, t })}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}

export function ApiKeyGroupCell({
  apiKey,
  groupOptions,
}: {
  apiKey: ApiKey
  groupOptions: ApiKeyGroupOption[]
}) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const mutation = useMutation({
    mutationFn: (group: string) =>
      updateApiKey(getGroupUpdatePayload(apiKey, group)),
    onSuccess: (result) => {
      if (result.success) {
        toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
        triggerRefresh()
        return
      }

      toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
    },
    onError: () => {
      toast.error(t(ERROR_MESSAGES.UPDATE_FAILED))
    },
  })

  const options = ensureCurrentGroupOption(groupOptions, apiKey, t('User Group'))

  return (
    <ApiKeyGroupCombobox
      options={options}
      value={apiKey.group || ''}
      onValueChange={(group) => {
        if (group === (apiKey.group || '')) return
        mutation.mutate(group)
      }}
      placeholder={t('Select a group')}
      disabled={mutation.isPending || options.length === 0}
      compact
    />
  )
}

export function ModelLimitsCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()

  if (!apiKey.model_limits_enabled || !apiKey.model_limits) {
    return (
      <StatusBadge
        label={t('Unlimited')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const models = apiKey.model_limits.split(',').filter(Boolean)

  return (
    <Tooltip>
      <TooltipTrigger render={<BadgeCell />}>
        <StatusBadge
          label={t('{{count}} model(s)', { count: models.length })}
          variant='neutral'
          copyable={false}
        />
      </TooltipTrigger>
      <TooltipContent side='top' className='max-w-xs'>
        <div className='max-h-[200px] space-y-0.5 overflow-y-auto text-xs'>
          {models.map((m) => (
            <div key={m} className='font-mono'>
              {m}
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}

export function IpRestrictionsCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const allowIps = apiKey.allow_ips?.trim()

  if (!allowIps) {
    return (
      <StatusBadge
        label={t('No restriction')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const ips = allowIps
    .split('\n')
    .map((ip) => ip.trim())
    .filter(Boolean)

  return (
    <Tooltip>
      <TooltipTrigger render={<BadgeCell />}>
        <StatusBadge
          label={t('{{count}} IP(s)', { count: ips.length })}
          variant='neutral'
          copyable={false}
        />
      </TooltipTrigger>
      <TooltipContent side='top' className='max-w-xs'>
        <div className='max-h-[200px] space-y-0.5 overflow-y-auto text-xs'>
          {ips.map((ip) => (
            <div key={ip} className='font-mono'>
              {ip}
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
