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
import { Save } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'

import { useUpdateOption } from '../hooks/use-update-option'

type ApiKeyNoticeSettingsProps = {
  value: string
}

const MAX_API_KEY_NOTICE_LENGTH = 500

export function ApiKeyNoticeSettings(props: ApiKeyNoticeSettingsProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [draft, setDraft] = useState(props.value)
  const [savedValue, setSavedValue] = useState(props.value)
  const draftLength = [...draft].length

  useEffect(() => {
    setDraft(props.value)
    setSavedValue(props.value)
  }, [props.value])

  const handleSave = async () => {
    try {
      const result = await updateOption.mutateAsync({
        key: 'console_setting.api_key_notice',
        value: draft,
      })
      if (result.success) setSavedValue(draft)
    } catch {
      // The shared mutation hook displays the request error.
    }
  }

  return (
    <>
      <FieldGroup className='gap-3'>
        <Field>
          <FieldLabel htmlFor='api-key-notice'>
            {t('API Key Notice')}
          </FieldLabel>
          <FieldDescription>
            {t('Displayed beside the API key filters.')}
          </FieldDescription>
          <Textarea
            id='api-key-notice'
            value={draft}
            rows={3}
            onChange={(event) =>
              setDraft(
                [...event.target.value]
                  .slice(0, MAX_API_KEY_NOTICE_LENGTH)
                  .join('')
              )
            }
          />
        </Field>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-xs tabular-nums'>
            {draftLength} / {MAX_API_KEY_NOTICE_LENGTH}
          </span>
          <Button
            type='button'
            size='sm'
            variant='secondary'
            disabled={draft === savedValue || updateOption.isPending}
            onClick={handleSave}
          >
            <Save data-icon='inline-start' aria-hidden='true' />
            {updateOption.isPending ? t('Saving...') : t('Save Settings')}
          </Button>
        </div>
      </FieldGroup>
      <Separator className='my-4' />
    </>
  )
}
