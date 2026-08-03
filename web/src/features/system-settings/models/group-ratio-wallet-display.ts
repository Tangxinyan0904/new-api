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
import { normalizeJsonString } from './utils'

export const GROUP_RATIO_WALLET_DISPLAY_OPTION =
  'group_ratio_setting.group_group_ratio_wallet_display'

export type WalletDisplayMap = Record<string, Record<string, true>>

type WalletDisplayOptionValues = {
  GroupGroupRatio: string
  GroupGroupRatioWalletDisplay: string
}

type WalletDisplayOptionUpdate = {
  key: 'GroupGroupRatio' | typeof GROUP_RATIO_WALLET_DISPLAY_OPTION
  value: string
}

export function parseWalletDisplayMap(value: string): WalletDisplayMap {
  const parsed: unknown = JSON.parse(value.trim() || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Wallet display rules must be a JSON object')
  }

  const result: WalletDisplayMap = {}
  for (const [userGroup, rawTargets] of Object.entries(parsed)) {
    if (
      !userGroup.trim() ||
      !rawTargets ||
      typeof rawTargets !== 'object' ||
      Array.isArray(rawTargets)
    ) {
      throw new Error('Wallet display groups must be non-empty JSON objects')
    }
    for (const [billingGroup, visible] of Object.entries(rawTargets)) {
      if (!billingGroup.trim() || typeof visible !== 'boolean') {
        throw new Error(
          'Wallet display entries must use non-empty groups and boolean values'
        )
      }
      if (!visible) continue
      result[userGroup] ??= {}
      result[userGroup][billingGroup] = true
    }
  }
  return result
}

export function serializeWalletDisplayMap(value: WalletDisplayMap): string {
  const sorted: WalletDisplayMap = {}
  for (const userGroup of Object.keys(value).sort()) {
    const targets = Object.keys(value[userGroup]).sort()
    if (targets.length === 0) continue
    sorted[userGroup] = {}
    for (const target of targets) {
      sorted[userGroup][target] = true
    }
  }
  return JSON.stringify(sorted, null, 2)
}

export function setWalletDisplayRule(
  value: string,
  userGroup: string,
  billingGroup: string,
  visible: boolean
): string {
  const map = parseWalletDisplayMap(value)
  if (visible) {
    map[userGroup] ??= {}
    map[userGroup][billingGroup] = true
  } else if (map[userGroup]) {
    delete map[userGroup][billingGroup]
    if (Object.keys(map[userGroup]).length === 0) {
      delete map[userGroup]
    }
  }
  return serializeWalletDisplayMap(map)
}

export function isWalletDisplayRuleSelected(
  value: string,
  userGroup: string,
  billingGroup: string
): boolean {
  return Boolean(parseWalletDisplayMap(value)[userGroup]?.[billingGroup])
}

export function moveWalletDisplayRule(
  value: string,
  oldUserGroup: string,
  oldBillingGroup: string,
  nextUserGroup: string,
  nextBillingGroup: string
): string {
  const wasSelected = isWalletDisplayRuleSelected(
    value,
    oldUserGroup,
    oldBillingGroup
  )
  const withoutOldRule = setWalletDisplayRule(
    value,
    oldUserGroup,
    oldBillingGroup,
    false
  )
  if (!wasSelected) return withoutOldRule
  return setWalletDisplayRule(
    withoutOldRule,
    nextUserGroup,
    nextBillingGroup,
    true
  )
}

export function validateWalletDisplayPairs(
  displayValue: string,
  ratioValue: string
): void {
  const display = parseWalletDisplayMap(displayValue)
  const ratios: unknown = JSON.parse(ratioValue.trim() || '{}')
  if (!ratios || typeof ratios !== 'object' || Array.isArray(ratios)) {
    throw new Error('Special ratio rules must be a JSON object')
  }

  for (const [userGroup, targets] of Object.entries(display)) {
    const userRatios = (ratios as Record<string, unknown>)[userGroup]
    if (
      !userRatios ||
      typeof userRatios !== 'object' ||
      Array.isArray(userRatios)
    ) {
      throw new Error(`Wallet display rule ${userGroup} has no special ratios`)
    }
    for (const billingGroup of Object.keys(targets)) {
      const ratio = (userRatios as Record<string, unknown>)[billingGroup]
      if (typeof ratio !== 'number' || !Number.isFinite(ratio)) {
        throw new Error(
          `Wallet display rule ${userGroup} -> ${billingGroup} has no special ratio`
        )
      }
    }
  }
}

export function buildWalletDisplayOptionUpdates(
  values: WalletDisplayOptionValues,
  defaults: WalletDisplayOptionValues
): WalletDisplayOptionUpdate[] {
  const normalizedValues = {
    GroupGroupRatio: normalizeJsonString(values.GroupGroupRatio),
    GroupGroupRatioWalletDisplay: serializeWalletDisplayMap(
      parseWalletDisplayMap(values.GroupGroupRatioWalletDisplay)
    ),
  }
  const normalizedDefaults = {
    GroupGroupRatio: normalizeJsonString(defaults.GroupGroupRatio),
    GroupGroupRatioWalletDisplay: serializeWalletDisplayMap(
      parseWalletDisplayMap(defaults.GroupGroupRatioWalletDisplay)
    ),
  }
  const updates: WalletDisplayOptionUpdate[] = []
  if (normalizedValues.GroupGroupRatio !== normalizedDefaults.GroupGroupRatio) {
    updates.push({
      key: 'GroupGroupRatio',
      value: normalizedValues.GroupGroupRatio,
    })
  }
  if (
    normalizedValues.GroupGroupRatioWalletDisplay !==
    normalizedDefaults.GroupGroupRatioWalletDisplay
  ) {
    updates.push({
      key: GROUP_RATIO_WALLET_DISPLAY_OPTION,
      value: normalizedValues.GroupGroupRatioWalletDisplay,
    })
  }
  return updates
}
