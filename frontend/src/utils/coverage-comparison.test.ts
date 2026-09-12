import { describe, expect, it } from 'vitest'
import {
  changedFields,
  fieldLabel,
  formatDelta,
  formatProtection,
  formatTime,
  reasonLabel,
  safeguardChangeLabel,
  shortHash,
  summarizeComparison,
} from './coverage-comparison'
import type { EvaluationComparison } from '../types/coverage-evaluation'

function comparison(overrides: Partial<EvaluationComparison> = {}): EvaluationComparison {
  return {
    base_id: 1,
    compared_id: 2,
    base: {
      id: 1, evaluated_at: '2026-08-01T09:00:00Z', algorithm_version: 'hazop-cover-v1.0.0',
      input_hash: 'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2',
      evaluation_state: 'completed', coverage_score: 20, risk_rank_before: 'high',
      risk_rank_after: 'high', evaluated_by_name: 'engineer',
    },
    compared: {
      id: 2, evaluated_at: '2026-09-01T10:30:00Z', algorithm_version: 'hazop-cover-v1.0.0',
      input_hash: 'f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5',
      evaluation_state: 'confirmed', coverage_score: 80, risk_rank_before: 'high',
      risk_rank_after: 'low', evaluated_by_name: 'reviewer',
    },
    score_delta: 60,
    uncovered_path_delta: -1,
    risk_rank_changed: true,
    input_changed: true,
    algorithm_changed: false,
    safeguard_changes: [
      { change_type: 'added', safeguard_id: 12, name: 'SIS-201', independence_key: 'SIS' },
      { change_type: 'removed', safeguard_id: 9, name: 'Old alarm', independence_key: 'ALM' },
      {
        change_type: 'changed', safeguard_id: 7, name: 'PSV-101', independence_key: 'PSV',
        field_changes: [{ field: 'effectiveness', before: '0.2000', after: '0.8000' }],
      },
    ],
    path_changes: [],
    added_uncovered_paths: [
      {
        change_type: 'added', path_id: 'P-002', node_code: 'C-101',
        cause: 'pump trip', consequence: 'dry run', reason_code: 'protection_lost',
        reason: 'independent protection became unavailable or weaker',
        before_combined_protection: 0.8, after_combined_protection: 0,
      },
    ],
    eliminated_uncovered_paths: [
      {
        change_type: 'eliminated', path_id: 'P-001', node_code: 'C-101',
        cause: 'cooling water loss', consequence: 'reactor overpressure',
        reason_code: 'protection_added',
        reason: 'newly available independent protection closed the gap',
        before_combined_protection: 0.2, after_combined_protection: 0.8,
      },
    ],
    ...overrides,
  }
}

describe('coverage comparison helpers', () => {
  it('summarizes safeguard and path changes', () => {
    const summary = summarizeComparison(comparison())
    expect(summary).toEqual({
      addedSafeguards: 1,
      removedSafeguards: 1,
      changedSafeguards: 1,
      addedPaths: 1,
      eliminatedPaths: 1,
    })
  })

  it('maps change and reason codes to Chinese labels', () => {
    expect(safeguardChangeLabel('added')).toBe('新增')
    expect(safeguardChangeLabel('removed')).toBe('移除')
    expect(safeguardChangeLabel('changed')).toBe('字段变化')
    expect(fieldLabel('effectiveness')).toBe('有效性')
    expect(reasonLabel('protection_added')).toBe('新增或增强保护层')
    expect(reasonLabel('scenario_changed')).toBe('场景原因/后果修订')
  })

  it('formats protection and signed score deltas', () => {
    expect(formatProtection(0.234)).toBe('23%')
    expect(formatProtection(undefined)).toBe('—')
    expect(formatDelta(12.34567)).toBe('+12.3457')
    expect(formatDelta(-1)).toBe('-1')
    expect(formatDelta(0)).toBe('0')
  })

  it('truncates hashes and parses RFC3339 times', () => {
    expect(shortHash('abcdef0123456789')).toBe('abcdef012345…')
    const formatted = formatTime('2026-09-01T10:30:00Z')
    expect(formatted).toContain('2026')
    expect(formatTime('')).toBe('—')
  })

  it('returns field changes defensively when absent', () => {
    expect(changedFields({})).toEqual([])
    expect(changedFields({ field_changes: comparison().safeguard_changes[2].field_changes })).toHaveLength(1)
  })
})
