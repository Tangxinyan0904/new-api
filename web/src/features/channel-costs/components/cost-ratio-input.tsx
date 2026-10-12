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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

import {
  MAX_CHANNEL_COST_RATIO,
  parseCostRatioInput,
} from '../lib/channel-cost-math'

interface CostRatioInputProps {
  /** Saved ratio; null shows the placeholder (the channel uses the default). */
  value: number | null
  id?: string
  label: string
  placeholder?: string
  /** Whether clearing the field is allowed (it saves null). */
  allowEmpty: boolean
  disabled?: boolean
  className?: string
  /** Resolves true when the ratio was saved. */
  onSave: (ratio: number | null) => Promise<boolean>
}

function ratioText(value: number | null): string {
  return value === null ? '' : String(value)
}

/** Saves on blur or Enter; Escape or a failed save restores the saved value. */
export function CostRatioInput(props: CostRatioInputProps) {
  const { t } = useTranslation()
  const [text, setText] = useState(ratioText(props.value))
  const [saving, setSaving] = useState(false)
  // Escape restores the saved value; the blur it triggers must not save.
  const cancelled = useRef(false)

  useEffect(() => {
    setText(ratioText(props.value))
  }, [props.value])

  const commit = async () => {
    if (cancelled.current) {
      cancelled.current = false
      setText(ratioText(props.value))
      return
    }
    const ratio = parseCostRatioInput(text)
    if (ratio === undefined || (ratio === null && !props.allowEmpty)) {
      toast.error(
        t('Cost ratio must be between 0 and {{max}}', {
          max: MAX_CHANNEL_COST_RATIO,
        })
      )
      setText(ratioText(props.value))
      return
    }
    if (ratio === props.value) {
      setText(ratioText(props.value))
      return
    }
    setSaving(true)
    const saved = await props.onSave(ratio)
    setSaving(false)
    if (!saved) setText(ratioText(props.value))
  }

  return (
    <Input
      id={props.id}
      type='number'
      inputMode='decimal'
      min={0}
      max={MAX_CHANNEL_COST_RATIO}
      step='0.01'
      value={text}
      placeholder={props.placeholder}
      aria-label={props.label}
      disabled={props.disabled || saving}
      className={cn('h-8 w-24 tabular-nums', props.className)}
      onChange={(event) => setText(event.target.value)}
      onBlur={() => void commit()}
      onKeyDown={(event) => {
        if (event.key === 'Enter') event.currentTarget.blur()
        if (event.key === 'Escape') {
          cancelled.current = true
          event.currentTarget.blur()
        }
      }}
    />
  )
}
