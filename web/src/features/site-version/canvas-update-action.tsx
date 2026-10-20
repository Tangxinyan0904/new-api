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
import { ArrowUp01Icon, PaintBoardIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { cn } from '@/lib/utils'

import { CanvasUpdateDialog } from './canvas-update-dialog'
import { useCanvasUpdate } from './use-canvas-update'

type CanvasUpdateActionProps = {
  /** 'version' sits next to the site name; 'action' is a full button. */
  presentation?: 'version' | 'action'
}

/**
 * The infinite canvas version, with a dialog that updates it. Only the root
 * user sees it, and only once the canvas updater is configured.
 */
export function CanvasUpdateAction(props: CanvasUpdateActionProps) {
  const { t } = useTranslation()
  const canvas = useCanvasUpdate()
  const [open, setOpen] = useState(false)
  const versionPresentation = props.presentation !== 'action'
  if (!canvas.isRoot || !canvas.status) return null
  if (!canvas.status.configured) {
    if (versionPresentation) return null
    return (
      <p className='text-muted-foreground text-sm'>
        {t(
          'Set CANVAS_UPDATER_URL and CANVAS_UPDATER_TOKEN for new-api to see and update the infinite canvas here.'
        )}
      </p>
    )
  }

  const version = canvas.status.current_version || t('Unknown version')
  const showUpdate = canvas.status.update_available && !canvas.updating
  let description = t('Infinite canvas, current version: {{version}}', {
    version,
  })
  if (showUpdate) description = `${description}\n${t('Update available')}`

  const icon = canvas.updating ? (
    <Spinner data-icon='inline-start' aria-hidden='true' />
  ) : (
    <HugeiconsIcon
      icon={showUpdate ? ArrowUp01Icon : PaintBoardIcon}
      data-icon='inline-start'
      className={cn(showUpdate && 'text-primary')}
      aria-hidden='true'
    />
  )

  return (
    <CanvasUpdateDialog
      open={open}
      onOpenChange={setOpen}
      canvas={canvas}
      trigger={
        <Button
          type='button'
          variant={versionPresentation ? 'ghost' : 'outline'}
          aria-label={description}
          title={description}
          aria-busy={canvas.updating}
          className={cn(
            'relative',
            versionPresentation &&
              'text-muted-foreground size-8 px-0 @min-[22rem]/system-brand:h-7 @min-[22rem]/system-brand:w-auto @min-[22rem]/system-brand:gap-1.5 @min-[22rem]/system-brand:px-1.5'
          )}
        >
          {icon}
          <span
            className={cn(
              'max-w-32 truncate font-mono text-xs',
              versionPresentation && 'hidden @min-[22rem]/system-brand:inline'
            )}
          >
            {t('Canvas {{version}}', { version })}
          </span>
          {showUpdate && (
            <Badge
              variant='secondary'
              className={cn(
                'bg-primary/10 text-primary h-5 px-1.5 text-[10px]',
                versionPresentation &&
                  'hidden @min-[30rem]/system-brand:inline-flex'
              )}
            >
              {t('Update available')}
            </Badge>
          )}
          {showUpdate && versionPresentation && (
            <Badge
              aria-hidden='true'
              className='absolute -end-0.5 -top-0.5 size-1.5 min-w-0 p-0 @min-[30rem]/system-brand:hidden'
            />
          )}
        </Button>
      }
    />
  )
}
