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
import { useId } from 'react'

import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface TicketOptionSelectProps {
  label: string
  value: string
  options: { value: string; label: string }[]
  onChange: (value: string) => void
  disabled?: boolean
}

/** A compact inline select with a visible label, for ticket handling controls. */
export function TicketOptionSelect(props: TicketOptionSelectProps) {
  const id = useId()
  return (
    <div className='flex items-center gap-2'>
      <Label htmlFor={id} className='text-muted-foreground text-xs'>
        {props.label}
      </Label>
      <Select
        items={props.options}
        value={props.value}
        disabled={props.disabled}
        onValueChange={(value) => {
          if (typeof value === 'string' && value !== props.value) {
            props.onChange(value)
          }
        }}
      >
        <SelectTrigger id={id} size='sm' className='min-w-32'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {props.options.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  )
}
