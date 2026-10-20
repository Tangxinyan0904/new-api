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
import { lazy, Suspense, useState, type ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'

import type { CanvasStatus } from './types'
import type { useCanvasUpdate } from './use-canvas-update'

const ReleaseMarkdown = lazy(() =>
  import('@/components/ui/markdown').then((module) => ({
    default: module.Markdown,
  }))
)

type CanvasUpdateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  trigger: ReactElement
  canvas: ReturnType<typeof useCanvasUpdate>
}

/** What the last finished update did, or null when none has finished. */
function useLastUpdateText(status: CanvasStatus | undefined): {
  text: string
  failed: boolean
} | null {
  const { t } = useTranslation()
  const task = status?.task
  if (!task || task.status === 'pending' || task.status === 'running') {
    return null
  }
  const time = formatTimestampToDate(task.updated_at)
  if (task.status === 'failed') {
    const error = task.result?.error
    return {
      failed: true,
      text: t('Last update failed {{time}}: {{error}}', {
        time,
        error: error ? t(error.key, error.params) : task.error,
      }),
    }
  }
  if (!task.result?.updated) {
    return {
      failed: false,
      text: t('Last update {{time}}: already up to date', { time }),
    }
  }
  return {
    failed: false,
    text: t('Last update {{time}}: {{from}} → {{to}}', {
      time,
      from: task.result.from_version || t('Unknown version'),
      to: task.result.to_version || t('Unknown version'),
    }),
  }
}

export function CanvasUpdateDialog(props: CanvasUpdateDialogProps) {
  const { t } = useTranslation()
  const [confirming, setConfirming] = useState(false)
  const canvas = props.canvas
  const status = canvas.status
  const release = status?.release
  const lastUpdate = useLastUpdateText(status)
  let current = status?.current_version || t('Unknown version')
  let statusText = ''
  if (canvas.updating) {
    current = t('Updating...')
    statusText = t('Updating the canvas. This usually takes a minute or two.')
  } else if (canvas.checking) {
    statusText = t('Checking updates...')
  } else if (status && !status.check_error) {
    statusText = status.update_available
      ? t('A new canvas version is available.')
      : t('The canvas is up to date.')
  }
  const canUpdate =
    Boolean(status?.update_available) &&
    !status?.check_error &&
    !canvas.updating

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        trigger={props.trigger}
        title={t('Infinite canvas updates')}
        bodyClassName='space-y-4'
        footerClassName='sm:flex-wrap'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              className='sm:me-auto'
              disabled={canvas.checking || canvas.updating}
              onClick={() => void canvas.checkNow()}
            >
              {canvas.checking ? t('Checking updates...') : t('Check again')}
            </Button>
            {release?.url && (
              <Button
                variant='outline'
                nativeButton={false}
                role='link'
                render={
                  <a
                    href={release.url}
                    target='_blank'
                    rel='noopener noreferrer'
                  />
                }
              >
                {t('Go to GitHub')}
              </Button>
            )}
            <Button
              type='button'
              disabled={!canUpdate}
              onClick={() => setConfirming(true)}
            >
              {canvas.updating ? t('Updating...') : t('Update now')}
            </Button>
          </>
        }
      >
        <dl className='grid min-w-0 gap-3 text-sm sm:grid-cols-2'>
          <div className='min-w-0'>
            <dt className='text-muted-foreground'>{t('Current version')}</dt>
            <dd className='font-medium break-all'>{current}</dd>
          </div>
          <div className='min-w-0'>
            <dt className='text-muted-foreground'>{t('Latest version')}</dt>
            <dd className='font-medium break-all'>
              {release?.version || t('Unknown')}
            </dd>
          </div>
          {release?.published_at && (
            <div>
              <dt className='text-muted-foreground'>{t('Published at')}</dt>
              <dd>
                {formatTimestampToDate(
                  Date.parse(release.published_at),
                  'milliseconds'
                )}
              </dd>
            </div>
          )}
        </dl>
        {statusText && (
          <p role='status' aria-live='polite' className='text-sm'>
            {statusText}
          </p>
        )}
        {status?.check_error && (
          <Alert variant='destructive'>
            <AlertDescription>
              {t(status.check_error.key, status.check_error.params)}
            </AlertDescription>
          </Alert>
        )}
        {lastUpdate && (
          <Alert variant={lastUpdate.failed ? 'destructive' : 'default'}>
            <AlertDescription>{lastUpdate.text}</AlertDescription>
          </Alert>
        )}
        {status?.release_error && (
          <p className='text-muted-foreground text-sm'>
            {t(status.release_error.key, status.release_error.params)}
          </p>
        )}
        {props.open && release && (
          <Suspense fallback={<Skeleton className='h-24 w-full' />}>
            {release.notes ? (
              <ReleaseMarkdown baseUrl={release.url || undefined}>
                {release.notes}
              </ReleaseMarkdown>
            ) : (
              <p className='text-muted-foreground text-sm'>
                {t('No release notes provided.')}
              </p>
            )}
          </Suspense>
        )}
      </Dialog>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t('Update the infinite canvas?')}
        desc={t(
          'The canvas container is recreated with the newest image. People using the canvas need to reload the page. A new version may change how canvases saved in the browser are stored, so read the release notes first.'
        )}
        confirmText={t('Update now')}
        isLoading={canvas.updating}
        handleConfirm={() => {
          void canvas.startUpdate().then((queued) => {
            if (queued) setConfirming(false)
          })
        }}
      />
    </>
  )
}
