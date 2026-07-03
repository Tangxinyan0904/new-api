/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useState, useEffect, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getSelf } from '@/lib/api'

import {
  createAffiliateTransferRequest,
  getAffiliateCode,
  getAffiliateRebateSummary,
} from '../api'
import { generateAffiliateLink } from '../lib'
import type { AffiliateRebateSummary } from '../types'

export function useAffiliate() {
  const [affiliateCode, setAffiliateCode] = useState<string>('')
  const [affiliateLink, setAffiliateLink] = useState<string>('')
  const [rebateSummary, setRebateSummary] =
    useState<AffiliateRebateSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [transferring, setTransferring] = useState(false)
  const { copyToClipboard } = useCopyToClipboard()

  const fetchAffiliateData = useCallback(async (background = false) => {
    try {
      if (background) {
        setRefreshing(true)
      } else {
        setLoading(true)
      }
      const [codeResponse, summaryResponse] = await Promise.all([
        getAffiliateCode(),
        getAffiliateRebateSummary(),
      ])

      if (codeResponse.success && codeResponse.data) {
        setAffiliateCode(codeResponse.data)
        setAffiliateLink(generateAffiliateLink(codeResponse.data))
      }

      if (summaryResponse.success && summaryResponse.data) {
        setRebateSummary(summaryResponse.data)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate data:', error)
    } finally {
      if (background) {
        setRefreshing(false)
      } else {
        setLoading(false)
      }
    }
  }, [])

  const refreshAffiliateData = useCallback(
    () => fetchAffiliateData(true),
    [fetchAffiliateData]
  )

  const copyAffiliateLink = useCallback(() => {
    copyToClipboard(affiliateLink)
  }, [affiliateLink, copyToClipboard])

  const transferQuota = useCallback(async (): Promise<boolean> => {
    try {
      setTransferring(true)
      const response = await createAffiliateTransferRequest()

      if (response.success) {
        toast.success(
          response.message || i18next.t('Transfer request submitted')
        )
        await Promise.all([getSelf(), refreshAffiliateData()])
        return true
      }

      toast.error(response.message || i18next.t('Transfer request failed'))
      return false
    } catch {
      toast.error(i18next.t('Transfer request failed'))
      return false
    } finally {
      setTransferring(false)
    }
  }, [refreshAffiliateData])

  useEffect(() => {
    fetchAffiliateData()
  }, [fetchAffiliateData])

  return {
    affiliateCode,
    affiliateLink,
    rebateSummary,
    loading,
    refreshing,
    transferring,
    copyAffiliateLink,
    transferQuota,
    refetch: refreshAffiliateData,
  }
}
