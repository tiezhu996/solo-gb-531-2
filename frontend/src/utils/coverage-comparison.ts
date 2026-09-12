import type {
  ComparisonChangeType,
  ComparisonFieldChange,
  EvaluationComparison,
  UncoveredPathComparisonChange,
  UncoveredPathReasonCode,
} from '../types/coverage-evaluation'

export const safeguardChangeLabels: Record<ComparisonChangeType, string> = {
  added: '新增',
  removed: '移除',
  changed: '字段变化',
}

export const pathChangeLabels: Record<'added' | 'eliminated', string> = {
  added: '新增未覆盖',
  eliminated: '已消除',
}

export const comparisonFieldLabels: Record<string, string> = {
  name: '名称',
  type: '保护类型',
  independence_key: '独立性键',
  effectiveness: '有效性',
  test_interval_days: '测试间隔(天)',
  last_verified_at: '最近验证时间',
  lifecycle_state: '生命周期状态',
  evidence_note: '证据说明',
}

const reasonLabels: Record<UncoveredPathReasonCode, string> = {
  protection_lost: '保护层失效或减弱',
  protection_added: '新增或增强保护层',
  scenario_changed: '场景原因/后果修订',
  algorithm_changed: '算法版本变化',
  below_threshold: '新版本保护度仍低于阈值',
  threshold_reached: '新版本保护度达到阈值',
}

export function safeguardChangeLabel(changeType: ComparisonChangeType): string {
  return safeguardChangeLabels[changeType] ?? changeType
}

export function fieldLabel(field: string): string {
  return comparisonFieldLabels[field] ?? field
}

export function reasonLabel(code: UncoveredPathReasonCode | string): string {
  return reasonLabels[code as UncoveredPathReasonCode] ?? code
}

export function formatProtection(value?: number): string {
  if (value === undefined || value === null) return '—'
  return `${Math.round(value * 100)}%`
}

export function formatDelta(value: number): string {
  if (value > 0) return `+${roundDelta(value)}`
  return `${roundDelta(value)}`
}

export function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function shortHash(hash: string): string {
  return hash.length > 12 ? `${hash.slice(0, 12)}…` : hash
}

function roundDelta(value: number): number {
  return Math.round(value * 10000) / 10000
}

export interface ComparisonSummary {
  addedSafeguards: number
  removedSafeguards: number
  changedSafeguards: number
  addedPaths: number
  eliminatedPaths: number
}

export function summarizeComparison(comparison: EvaluationComparison): ComparisonSummary {
  return {
    addedSafeguards: comparison.safeguard_changes.filter((x) => x.change_type === 'added').length,
    removedSafeguards: comparison.safeguard_changes.filter((x) => x.change_type === 'removed').length,
    changedSafeguards: comparison.safeguard_changes.filter((x) => x.change_type === 'changed').length,
    addedPaths: comparison.added_uncovered_paths.length,
    eliminatedPaths: comparison.eliminated_uncovered_paths.length,
  }
}

export function changedFields(change: { field_changes?: ComparisonFieldChange[] }): ComparisonFieldChange[] {
  return change.field_changes ?? []
}

export function pathIdentity(path: Pick<UncoveredPathComparisonChange, 'node_code' | 'cause' | 'consequence'>): string {
  return [path.node_code, path.cause, path.consequence].map((x) => x.trim().toLowerCase()).join('|')
}
