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
import { Ellipsis } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { GroupBadge, GroupMultiplierBadge } from '@/components/group-badge'
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from '@/components/ui/hover-card'

import { getGroupRatios } from '../api'

/**
 * The groups of a channel in a table cell: the first one and a mark when there
 * are more. Hovering lists all of them with the ratio each charges, and the
 * special ratios set for users of particular groups.
 */
export function ChannelGroupsCell(props: {
  groups: string[]
  /** Shown in place of every group name while sensitive data is hidden. */
  mask?: string
}) {
  const { t } = useTranslation()
  const ratiosQuery = useQuery({
    queryKey: ['groups', 'ratios'],
    queryFn: getGroupRatios,
  })

  const firstGroup = props.groups[0]
  if (firstGroup === undefined) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }
  const ratios = ratiosQuery.data
  const moreCount = props.groups.length - 1

  return (
    <HoverCard>
      <HoverCardTrigger
        delay={150}
        closeDelay={100}
        render={
          <div
            tabIndex={0}
            className='-ml-1.5 flex max-w-full min-w-0 items-center gap-1'
          />
        }
      >
        <GroupBadge group={firstGroup} label={props.mask} size='sm' />
        {moreCount > 0 ? (
          <span
            data-slot='more-groups'
            className='bg-muted text-muted-foreground inline-flex h-5 shrink-0 items-center gap-0.5 rounded-full px-1.5 text-xs tabular-nums'
          >
            <Ellipsis aria-hidden='true' className='size-3' />+{moreCount}
          </span>
        ) : null}
      </HoverCardTrigger>
      <HoverCardContent
        align='start'
        className='max-h-64 w-auto max-w-80 min-w-44 overflow-y-auto p-2 text-xs'
      >
        <ul className='flex min-w-0 flex-col gap-2'>
          {props.groups.map((group) => (
            <li key={group} className='flex min-w-0 flex-col gap-1'>
              <GroupBadge
                group={group}
                label={props.mask}
                size='sm'
                ratio={ratios?.group_ratio[group]}
                containerClassName='justify-between'
              />
              {Object.entries(ratios?.group_group_ratio ?? {}).map(
                ([userGroup, billedGroups]) =>
                  billedGroups[group] === undefined ? null : (
                    <div
                      key={userGroup}
                      data-slot='special-group-ratio'
                      className='text-muted-foreground flex min-w-0 items-center justify-between gap-2 pl-4'
                    >
                      <span className='truncate'>
                        {t('{{group}} users', {
                          group: props.mask ?? userGroup,
                        })}
                      </span>
                      <GroupMultiplierBadge ratio={billedGroups[group]} />
                    </div>
                  )
              )}
            </li>
          ))}
        </ul>
      </HoverCardContent>
    </HoverCard>
  )
}
