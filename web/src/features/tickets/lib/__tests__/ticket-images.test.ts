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
import { describe, expect, it } from 'vitest'

import { TICKET_LIMITS } from '../../constants'
import { addTicketImages, pastedImages } from '../ticket-images'

function image(name: string, type = 'image/png', size = 1024): File {
  return new File([new Uint8Array(size)], name, { type })
}

describe('addTicketImages', () => {
  it('appends supported images to the current selection', () => {
    const current = [image('a.png')]
    const result = addTicketImages(current, [
      image('b.jpg', 'image/jpeg'),
      image('c.webp', 'image/webp'),
    ])

    expect(result.accepted.map((file) => file.name)).toEqual([
      'a.png',
      'b.jpg',
      'c.webp',
    ])
    expect(result.error).toBeUndefined()
  })

  it('rejects SVG and GIF files with the supported type message', () => {
    const result = addTicketImages(
      [],
      [image('x.svg', 'image/svg+xml'), image('y.gif', 'image/gif')]
    )

    expect(result.accepted).toEqual([])
    expect(result.error).toEqual({
      key: 'Only PNG, JPEG, and WebP images are supported',
    })
  })

  it('rejects images over the size limit and keeps the valid ones', () => {
    const result = addTicketImages(
      [],
      [
        image('big.png', 'image/png', TICKET_LIMITS.MAX_IMAGE_BYTES + 1),
        image('ok.png'),
      ]
    )

    expect(result.accepted.map((file) => file.name)).toEqual(['ok.png'])
    expect(result.error).toEqual({
      key: 'Each image must be {{max}} MB or smaller',
      params: { max: 2 },
    })
  })

  it('stops at the image count limit and reports the overflow', () => {
    const result = addTicketImages(
      [image('1.png'), image('2.png')],
      [image('3.png'), image('4.png')]
    )

    expect(result.accepted.map((file) => file.name)).toEqual([
      '1.png',
      '2.png',
      '3.png',
    ])
    expect(result.error).toEqual({
      key: 'You can attach at most {{max}} images',
      params: { max: TICKET_LIMITS.MAX_IMAGES },
    })
  })
})

describe('pastedImages', () => {
  it('keeps only image files from the clipboard', () => {
    // jsdom has no DataTransfer; the helper only reads the iterable files.
    const clipboardData = {
      files: [
        image('shot.png'),
        new File(['text'], 'notes.txt', { type: 'text/plain' }),
      ],
    } as unknown as DataTransfer

    expect(pastedImages({ clipboardData }).map((file) => file.name)).toEqual([
      'shot.png',
    ])
  })
})
