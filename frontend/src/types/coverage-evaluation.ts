import type { CoverageState } from './enums/coverage-state'

export interface PathEvidence {
  path_id?: string
  node_code?: string
  cause: string
  consequence: string
  safeguard_ids?: number[]
  safeguard_names?: string[]
  independence_keys?: string[]
  combined_protection?: number
  covered?: boolean
  protected?: boolean
  path_score?: number
  reason?: string
}

export interface ScoringStep {
  step?: number
  rule?: string
  input?: string
  contribution?: number
  running_score?: number
  explanation?: string
  label?: string
  value?: number | string
  detail?: string
}

export interface DeduplicatedSafeguard {
  independence_key: string
  kept_id: number
  ignored_ids: number[]
  reason: string
}

export interface EvaluationExplanation {
  summary: string
  paths: PathEvidence[]
  score_steps: ScoringStep[]
  deduplicated_safeguards: DeduplicatedSafeguard[]
  boundary_note: string
  reference_time: string
}

export interface CoverageSnapshot {
  scenario_id?: number
  scenario_version?: number
  causes?: string[]
  consequences?: string[]
  safeguards?: unknown[]
  input_hash?: string
}

export interface CoverageEvaluation {
  id: number
  scenario_id: number
  algorithm_version: string
  input_snapshot: CoverageSnapshot | string
  coverage_score: number
  uncovered_paths: PathEvidence[] | string
  risk_rank_before: string
  risk_rank_after: string
  evaluation_state: CoverageState
  explanation: EvaluationExplanation | string
  evaluated_by: number
  evaluated_by_name?: string
  evaluated_at: string
  input_hash?: string
  deduplicated_safeguards?: DeduplicatedSafeguard[]
  duration_milliseconds?: number
  determinism_replay_passed?: boolean
}

export interface CoverageRunInput { scenario_id: number }

export interface EvaluationVersionSummary {
  id: number
  evaluated_at: string
  algorithm_version: string
  input_hash: string
  evaluation_state: CoverageState
  coverage_score: number
  risk_rank_before: string
  risk_rank_after: string
  evaluated_by_name: string
}

export type ComparisonChangeType = 'added' | 'removed' | 'changed'
export type UncoveredPathChangeType = 'added' | 'eliminated'

export interface ComparisonFieldChange {
  field: string
  before: string
  after: string
}

export interface SafeguardComparisonChange {
  change_type: ComparisonChangeType
  safeguard_id: number
  name: string
  independence_key: string
  field_changes?: ComparisonFieldChange[]
}

export type UncoveredPathReasonCode =
  | 'protection_lost'
  | 'protection_added'
  | 'scenario_changed'
  | 'algorithm_changed'
  | 'below_threshold'
  | 'threshold_reached'

export interface UncoveredPathComparisonChange {
  change_type: UncoveredPathChangeType
  path_id: string
  node_code: string
  cause: string
  consequence: string
  reason_code: UncoveredPathReasonCode | string
  reason: string
  before_combined_protection?: number
  after_combined_protection?: number
  safeguard_ids?: number[]
  independence_keys?: string[]
}

export interface EvaluationComparison {
  base_id: number
  compared_id: number
  base: EvaluationVersionSummary
  compared: EvaluationVersionSummary
  score_delta: number
  uncovered_path_delta: number
  risk_rank_changed: boolean
  input_changed: boolean
  algorithm_changed: boolean
  safeguard_changes: SafeguardComparisonChange[]
  path_changes: UncoveredPathComparisonChange[]
  added_uncovered_paths: UncoveredPathComparisonChange[]
  eliminated_uncovered_paths: UncoveredPathComparisonChange[]
}
