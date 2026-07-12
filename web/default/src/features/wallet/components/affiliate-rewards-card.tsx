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
import { RefreshCw, Share2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { formatQuota } from '@/lib/format'

import type { AffiliateRebateSummary, UserWalletData } from '../types'

interface AffiliateRewardsCardProps {
  user: UserWalletData | null
  affiliateLink: string
  rebateSummary: AffiliateRebateSummary | null
  onRefresh: () => void | Promise<void>
  onTransfer: () => void
  complianceConfirmed?: boolean
  loading?: boolean
  refreshing?: boolean
  transferring?: boolean
}

const DEFAULT_PROMOTION_TEXT =
  '\u53d1\u73b0\u4e00\u4e2a\u8d85\u4f4e\u4ef7\u4e2d\u8f6c\uff0c\u9080\u8bf7\u6ce8\u518c\u90011\u5200\uff0c\u8fdb\u7fa4\u98861\u5200\uff0c\u8d85\u4f4e\u500d\u7387\u7b49\u4ef7\u522b\u4eba\u51e0\u5341\u5200\uff01'

export function AffiliateRewardsCard({
  user,
  affiliateLink,
  rebateSummary,
  onRefresh,
  onTransfer,
  complianceConfirmed = true,
  loading,
  refreshing,
  transferring,
}: AffiliateRewardsCardProps) {
  const { t } = useTranslation()
  const [promotionText, setPromotionText] = useState(DEFAULT_PROMOTION_TEXT)

  if (loading) {
    return (
      <Card data-card-hover='false' className='bg-muted/20 py-0'>
        <CardContent className='flex flex-col gap-4 p-4 sm:p-5'>
          <div className='flex items-center gap-3'>
            <Skeleton className='size-9 rounded-lg' />
            <div>
              <Skeleton className='h-5 w-32' />
              <Skeleton className='mt-2 h-4 w-56' />
            </div>
          </div>
          <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
            {[1, 2, 3, 4].map((item) => (
              <Skeleton key={item} className='h-14 rounded-lg' />
            ))}
          </div>
          <div className='grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(280px,0.7fr)]'>
            <Skeleton className='h-28 rounded-lg' />
            <Skeleton className='h-28 rounded-lg' />
          </div>
        </CardContent>
      </Card>
    )
  }

  const inviteReward =
    rebateSummary?.invite_reward_quota ?? user?.aff_quota ?? 0
  const rechargeRebate = rebateSummary?.recharge_rebate_quota ?? 0
  const totalPending = rebateSummary?.total_pending_quota ?? inviteReward
  const hasRewards = totalPending > 0
  const pendingRequest = Boolean(rebateSummary?.pending_request)
  const submittedToday = rebateSummary?.submitted_today ?? false
  const transferDisabled =
    !complianceConfirmed ||
    !hasRewards ||
    pendingRequest ||
    submittedToday ||
    transferring
  let transferLabel = t('Request Transfer')
  if (pendingRequest) {
    transferLabel = t('Pending Approval')
  } else if (submittedToday) {
    transferLabel = t('Submitted Today')
  }
  const invitedUsers = rebateSummary?.invited_users ?? []
  const promotionCopyValue = [promotionText.trim(), affiliateLink]
    .filter(Boolean)
    .join('\n')

  return (
    <Card data-card-hover='false' className='bg-muted/20 py-0'>
      <CardContent className='flex flex-col gap-4 p-4 sm:p-5'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div className='flex min-w-0 items-center gap-3'>
            <IconBadge tone='chart-3' size='title'>
              <Share2 />
            </IconBadge>
            <div className='min-w-0'>
              <h3 className='truncate text-sm font-semibold'>
                {t('Referral Program')}
              </h3>
              <p className='text-muted-foreground line-clamp-2 text-xs'>
                {t(
                  'Invitation rewards and recharge rebates are separated. Transfer requests require admin approval.'
                )}
              </p>
            </div>
          </div>
          <div className='flex shrink-0 items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void onRefresh()}
              disabled={refreshing}
              className='h-9 px-3'
            >
              <RefreshCw
                data-icon='inline-start'
                className={refreshing ? 'animate-spin' : undefined}
              />
              {refreshing ? t('Refreshing...') : t('Refresh')}
            </Button>
            <Button
              type='button'
              size='sm'
              onClick={onTransfer}
              disabled={transferDisabled}
              className='h-9 px-3'
            >
              {transferLabel}
            </Button>
          </div>
        </div>

        <div className='grid grid-cols-2 gap-2 text-center sm:grid-cols-4'>
          {[
            [t('Invitation Reward'), formatQuota(inviteReward)],
            [t('Recharge Rebate'), formatQuota(rechargeRebate)],
            [
              t('Invited Recharge'),
              formatQuota(rebateSummary?.total_invited_recharge_quota ?? 0),
            ],
            [t('Pending'), formatQuota(totalPending)],
          ].map(([label, value]) => (
            <div
              key={label}
              className='bg-background/60 rounded-md border px-2 py-2'
            >
              <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
                {label}
              </div>
              <div className='mt-1 truncate text-sm font-semibold tabular-nums'>
                {value}
              </div>
            </div>
          ))}
        </div>

        <div className='grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(280px,0.7fr)]'>
          <div className='bg-background/60 flex flex-col gap-3 rounded-md border p-3'>
            <div className='flex flex-col gap-1.5'>
              <Label htmlFor='affiliate-promotion-text' className='text-xs'>
                {t('Promotion Text')}
              </Label>
              <Textarea
                id='affiliate-promotion-text'
                value={promotionText}
                onChange={(event) => setPromotionText(event.target.value)}
                className='bg-background/70 min-h-18 resize-none text-sm'
              />
            </div>

            <div className='flex flex-col gap-1.5'>
              <Label htmlFor='affiliate-link' className='text-xs'>
                {t('Your Referral Link')}
              </Label>
              <Input
                id='affiliate-link'
                value={affiliateLink}
                readOnly
                className='border-muted bg-background/70 h-9 min-w-0 font-mono text-xs'
              />
            </div>

            <div className='flex flex-wrap items-center gap-2'>
              <CopyButton
                value={promotionCopyValue}
                variant='outline'
                size='sm'
                className='bg-background h-9 px-3'
                iconClassName='size-4'
                tooltip={t('Copy with promotion text')}
                aria-label={t('Copy with promotion text')}
              >
                {t('Copy with promotion text')}
              </CopyButton>
              <CopyButton
                value={affiliateLink}
                variant='outline'
                size='sm'
                className='bg-background h-9 px-3'
                iconClassName='size-4'
                tooltip={t('Copy referral link')}
                aria-label={t('Copy referral link')}
              >
                {t('Copy')}
              </CopyButton>
            </div>
          </div>

          <div className='bg-background/60 rounded-md border p-3'>
            <div className='mb-2 flex items-center justify-between gap-2'>
              <span className='text-muted-foreground text-xs'>
                {t('Invited Users')}
              </span>
              <span className='text-xs font-medium tabular-nums'>
                {rebateSummary?.invited_count ?? user?.aff_count ?? 0}
              </span>
            </div>
            <div className='flex flex-wrap items-center gap-1.5'>
              {invitedUsers.slice(0, 6).map((invited) => (
                <StatusBadge
                  key={invited.id}
                  label={invited.display_name}
                  variant='neutral'
                  copyable={false}
                />
              ))}
              {invitedUsers.length > 6 ? (
                <StatusBadge
                  label={`+${invitedUsers.length - 6}`}
                  variant='neutral'
                  copyable={false}
                />
              ) : null}
            </div>
          </div>
        </div>

        {!complianceConfirmed ? (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Referral reward transfer is disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
