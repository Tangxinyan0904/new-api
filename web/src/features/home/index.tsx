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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/context/theme-provider'
import { DEFAULT_GEOIP_POPUP_MESSAGE } from '@/lib/constants'
import { isLikelyHtml } from '@/lib/content-format'
import { useAuthStore } from '@/stores/auth-store'

import { CTA, Features, Hero, HowItWorks, Stats } from './components'
import { getGeoIPStatus } from './api'
import { useHomePageContent } from './hooks'
import type { GeoIPStatus } from './types'

export function Home() {
  const { i18n, t } = useTranslation()
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const { resolvedTheme } = useTheme()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent()
  const [geoIPStatus, setGeoIPStatus] = useState<GeoIPStatus | null>(null)
  const [geoIPDismissed, setGeoIPDismissed] = useState(false)

  const syncIframePreferences = useCallback(() => {
    try {
      iframeRef.current?.contentWindow?.postMessage(
        { themeMode: resolvedTheme },
        '*'
      )
      iframeRef.current?.contentWindow?.postMessage(
        { lang: i18n.language },
        '*'
      )
    } catch {
      // Cross-origin frames may reject access while navigating.
    }
  }, [i18n.language, resolvedTheme])

  useEffect(() => {
    if (isUrl) {
      syncIframePreferences()
    }
  }, [isUrl, syncIframePreferences])

  useEffect(() => {
    let mounted = true

    const loadGeoIPStatus = async () => {
      try {
        const response = await getGeoIPStatus()
        if (mounted && response.success && response.data) {
          setGeoIPStatus(response.data)
        }
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to load GeoIP status:', error)
      }
    }

    loadGeoIPStatus()

    return () => {
      mounted = false
    }
  }, [])

  const showGeoIPDialog =
    !!geoIPStatus?.enabled &&
    !!geoIPStatus.blocked &&
    geoIPStatus.mode !== 'full_reject' &&
    !geoIPDismissed
  const geoIPDialogDismissible = geoIPStatus?.mode === 'homepage_notice'
  const geoIPMessage =
    !geoIPStatus?.message ||
    geoIPStatus.message === DEFAULT_GEOIP_POPUP_MESSAGE
      ? t(DEFAULT_GEOIP_POPUP_MESSAGE)
      : geoIPStatus.message

  const geoIPDialog = (
    <Dialog
      open={showGeoIPDialog}
      onOpenChange={(open) => {
        if (!open && geoIPDialogDismissible) {
          setGeoIPDismissed(true)
        }
      }}
      title={t('GeoIP access notice')}
      showCloseButton={geoIPDialogDismissible}
      contentClassName='sm:max-w-md'
      footer={
        geoIPDialogDismissible ? (
          <Button type='button' onClick={() => setGeoIPDismissed(true)}>
            {t('I understand')}
          </Button>
        ) : null
      }
    >
      <p className='text-muted-foreground text-sm leading-6'>
        {geoIPMessage}
      </p>
    </Dialog>
  )

  if (!isLoaded) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='flex min-h-screen items-center justify-center'>
          <div className='text-muted-foreground'>{t('Loading...')}</div>
        </main>
        {geoIPDialog}
      </PublicLayout>
    )
  }

  if (content) {
    if (isUrl) {
      return (
        <PublicLayout showMainContainer={false}>
          {/*
            allow-top-navigation-by-user-activation: the custom home page URL is
            admin-configured (trusted); this lets its target="_top" nav/menu links
            navigate the top-level window on user click. The default sandbox blocks
            this on desktop, while some mobile browsers allow it via allow-popups,
            causing inconsistent behavior. This token only permits user-activated
            top-level navigation and does NOT grant same-origin access.
          */}
          <iframe
            ref={iframeRef}
            src={content}
            className='h-screen w-full border-none'
            title={t('Custom Home Page')}
            sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts allow-top-navigation-by-user-activation'
            onLoad={syncIframePreferences}
          />
          {geoIPDialog}
        </PublicLayout>
      )
    }

    const contentIsHtml = isLikelyHtml(content)

    if (contentIsHtml) {
      return (
        <PublicLayout showMainContainer={false}>
          <RichContent
            mode='html'
            htmlVariant='isolated'
            content={content}
            className='custom-home-content'
          />
          {geoIPDialog}
        </PublicLayout>
      )
    }

    return (
      <PublicLayout>
        <div className='mx-auto max-w-6xl px-4 py-8'>
          <RichContent
            mode='markdown'
            content={content}
            className='custom-home-content'
          />
        </div>
        {geoIPDialog}
      </PublicLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <Hero isAuthenticated={isAuthenticated} />
      <Stats />
      <Features />
      <HowItWorks />
      <CTA isAuthenticated={isAuthenticated} />
      <Footer />
      {geoIPDialog}
    </PublicLayout>
  )
}
