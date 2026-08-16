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

const REFRESH_INTERVAL_SECONDS = 60

export function useChannelStatusRefresh(onRefresh: () => Promise<void>) {
  const onRefreshRef = useRef(onRefresh)
  const secondsRemainingRef = useRef(REFRESH_INTERVAL_SECONDS)
  const refreshingRef = useRef(false)
  const [enabled, setEnabled] = useState(true)
  const [secondsRemaining, setSecondsRemaining] = useState(
    REFRESH_INTERVAL_SECONDS
  )
  const [isRefreshing, setIsRefreshing] = useState(false)

  useEffect(() => {
    onRefreshRef.current = onRefresh
  }, [onRefresh])

  const resetCountdown = useCallback(() => {
    secondsRemainingRef.current = REFRESH_INTERVAL_SECONDS
    setSecondsRemaining(REFRESH_INTERVAL_SECONDS)
  }, [])

  const refreshNow = useCallback(async () => {
    if (refreshingRef.current) return

    refreshingRef.current = true
    setIsRefreshing(true)

    try {
      await onRefreshRef.current()
    } finally {
      refreshingRef.current = false
      setIsRefreshing(false)
      resetCountdown()
    }
  }, [resetCountdown])

  useEffect(() => {
    if (!enabled) return

    const intervalId = globalThis.setInterval(() => {
      const nextSeconds = secondsRemainingRef.current - 1

      if (nextSeconds <= 0) {
        resetCountdown()
        void refreshNow()
        return
      }

      secondsRemainingRef.current = nextSeconds
      setSecondsRemaining(nextSeconds)
    }, 1000)

    return () => globalThis.clearInterval(intervalId)
  }, [enabled, refreshNow, resetCountdown])

  const toggle = useCallback(() => {
    resetCountdown()
    setEnabled((current) => !current)
  }, [resetCountdown])

  return {
    enabled,
    secondsRemaining,
    isRefreshing,
    toggle,
    refreshNow,
  }
}
