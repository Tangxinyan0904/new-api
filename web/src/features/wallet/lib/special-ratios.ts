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
export type SpecialRatioCardDisplay = 'loading' | 'error' | 'hidden' | 'rules'

export function getSpecialRatioSummary(input: {
  billingGroup: string
  baseRatio: number
  specialRatio: number
}): {
  currentRatio: number
  upgradeGroup: string
  upgradeRatio: number
} {
  return {
    currentRatio: input.baseRatio,
    upgradeGroup: input.billingGroup,
    upgradeRatio: input.specialRatio,
  }
}

export function getSpecialRatioCardState(input: {
  isPending: boolean
  isError: boolean
  count: number
}): { display: SpecialRatioCardDisplay; available: boolean } {
  if (input.isPending) return { display: 'loading', available: true }
  if (input.isError) return { display: 'error', available: true }
  if (input.count === 0) return { display: 'hidden', available: false }
  return { display: 'rules', available: true }
}

export function getWalletPrimaryGridClass(
  subscriptionAvailable: boolean,
  specialRatiosAvailable: boolean
): string {
  if (subscriptionAvailable && specialRatiosAvailable) {
    return 'grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3 2xl:items-stretch'
  }
  if (subscriptionAvailable || specialRatiosAvailable) {
    return 'grid grid-cols-1 gap-4 xl:grid-cols-2 xl:items-stretch'
  }
  return 'grid grid-cols-1 gap-4'
}
