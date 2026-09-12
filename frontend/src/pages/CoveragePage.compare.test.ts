import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus, { ElOption } from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import type { EvaluationComparison } from '../types/coverage-evaluation'
import type { DeviationScenario } from '../types/deviation-scenario'
import type { Safeguard } from '../types/safeguard'
import type { CoverageEvaluation } from '../types/coverage-evaluation'

const compareMock = vi.fn()
const listEvaluationsMock = vi.fn()

vi.mock('../api/coverage-evaluation', () => ({
  listCoverageEvaluations: (...args: unknown[]) => listEvaluationsMock(...args),
  getCoverageEvaluation: vi.fn(),
  runCoverageEvaluation: vi.fn(),
  confirmCoverageEvaluation: vi.fn(),
  voidCoverageEvaluation: vi.fn(),
  replayCoverageEvaluation: vi.fn(),
  compareCoverageEvaluations: (...args: unknown[]) => compareMock(...args),
}))

vi.mock('../api/deviation-scenario', () => ({
  listDeviationScenarios: vi.fn(async () => ({ items: [scenario()], total: 1 })),
  getDeviationScenario: vi.fn(),
  createDeviationScenario: vi.fn(),
  updateDeviationScenario: vi.fn(),
  transitionDeviationScenario: vi.fn(),
}))

vi.mock('../api/safeguard', () => ({
  listSafeguards: vi.fn(async () => ({ items: [safeguard()], total: 1 })),
  getSafeguard: vi.fn(),
  createSafeguard: vi.fn(),
  updateSafeguard: vi.fn(),
  verifySafeguard: vi.fn(),
  invalidateSafeguard: vi.fn(),
  restoreSafeguard: vi.fn(),
}))

vi.mock('../hooks/useAuth', () => ({
  useAuth: () => ({
    canEdit: { value: true },
    canReview: { value: true },
    isAuditor: { value: false },
    user: { value: { username: 'engineer' } },
  }),
}))

vi.mock('../hooks/useCoverageRun', () => ({
  useCoverageRun: () => ({
    running: { value: false },
    polling: { value: false },
    launch: vi.fn(),
    stop: vi.fn(),
  }),
}))

import CoveragePage from './CoveragePage.vue'
import { useCoverageEvaluationStore } from '../stores/coverage-evaluation'

function scenario(): DeviationScenario {
  return {
    id: 11,
    process_node_id: 1,
    guideword: 'more',
    parameter: 'temperature',
    cause: 'Cooling water flow lost',
    consequence: 'Reactor overpressure',
    likelihood: 4,
    severity: 5,
    risk_score: 20,
    risk_rank: 'critical',
    scenario_state: 'analyzed',
    version: 2,
    created_by: 1,
    created_by_name: 'engineer',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z',
  } as DeviationScenario
}

function safeguard(): Safeguard {
  return {
    id: 1,
    name: 'High temperature SIS trip',
    safeguard_type: 'interlock',
    target_scenario_id: 11,
    independence_key: 'SIS-R101-TEMP',
    effectiveness: 0.8,
    test_interval_days: 365,
    last_verified_at: '2026-08-23T00:00:00Z',
    lifecycle_state: 'active',
    evidence_note: 'cert',
  }
}

function evaluation(overrides: Partial<CoverageEvaluation>): CoverageEvaluation {
  return {
    id: 0,
    scenario_id: 11,
    algorithm_version: 'hazop-cover-v1.0.0',
    input_snapshot: '',
    coverage_score: 0,
    uncovered_paths: [],
    risk_rank_before: 'critical',
    risk_rank_after: 'low',
    evaluation_state: 'completed',
    explanation: {
      summary: '',
      paths: [],
      score_steps: [],
      deduplicated_safeguards: [],
      boundary_note: '',
      reference_time: '2026-09-01T00:00:00Z',
    },
    evaluated_by: 2,
    evaluated_by_name: 'engineer',
    evaluated_at: '2026-09-01T00:00:00Z',
    input_hash: 'x',
    ...overrides,
  } as CoverageEvaluation
}

const evaluations: CoverageEvaluation[] = [
  evaluation({ id: 2, coverage_score: 45, risk_rank_after: 'high', evaluation_state: 'completed' }),
  evaluation({ id: 1, coverage_score: 80, risk_rank_after: 'low', evaluation_state: 'completed' }),
  evaluation({ id: 3, coverage_score: 0, risk_rank_after: 'high', evaluation_state: 'running' }),
  evaluation({ id: 4, scenario_id: 22, coverage_score: 70, evaluation_state: 'completed' }),
]

function comparisonPayload(): EvaluationComparison {
  return {
    base_id: 1,
    compared_id: 2,
    base: {
      id: 1, evaluated_at: '2026-08-01T09:00:00Z', algorithm_version: 'hazop-cover-v1.0.0',
      input_hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
      evaluation_state: 'completed', coverage_score: 80, risk_rank_before: 'critical',
      risk_rank_after: 'low', evaluated_by_name: 'engineer',
    },
    compared: {
      id: 2, evaluated_at: '2026-09-01T10:00:00Z', algorithm_version: 'hazop-cover-v1.0.0',
      input_hash: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
      evaluation_state: 'completed', coverage_score: 45, risk_rank_before: 'critical',
      risk_rank_after: 'high', evaluated_by_name: 'engineer',
    },
    score_delta: -35,
    uncovered_path_delta: 4,
    risk_rank_changed: true,
    input_changed: true,
    algorithm_changed: false,
    safeguard_changes: [
      {
        change_type: 'changed', safeguard_id: 1, name: 'High temperature SIS trip',
        independence_key: 'SIS-R101-TEMP',
        field_changes: [
          { field: 'effectiveness', before: '0.8000', after: '0.4500' },
          { field: 'lifecycle_state', before: 'active', after: 'invalid' },
        ],
      },
      { change_type: 'removed', safeguard_id: 9, name: 'Old alarm', independence_key: 'ALM' },
    ],
    path_changes: [],
    added_uncovered_paths: Array.from({ length: 4 }, (_, index) => ({
      change_type: 'added' as const,
      path_id: `P-00${index + 1}`,
      node_code: 'R-101',
      cause: 'Cooling water flow lost',
      consequence: 'Reactor overpressure',
      reason_code: 'protection_lost',
      reason: 'independent protection became unavailable or weaker',
      before_combined_protection: 0.8,
      after_combined_protection: 0.45,
    })),
    eliminated_uncovered_paths: [],
  }
}

function mountPage() {
  const wrapper = mount(CoveragePage, {
    global: {
      plugins: [ElementPlus],
      stubs: {
        AppShell: { template: '<div><slot /></div>' },
        PageHeader: { template: '<div><slot name="default" /><slot /></div>' },
        ScenarioStateTimeline: true,
        EvidenceDrawer: true,
      },
    },
  })
  return wrapper
}

function compareSelect(wrapper: ReturnType<typeof mountPage>) {
  return wrapper.find('.compare-tools').findComponent({ name: 'ElSelect' })
}

async function chooseHistorical(wrapper: ReturnType<typeof mountPage>, id: number | undefined) {
  await compareSelect(wrapper).vm.$emit('update:modelValue', id)
  await flushPromises()
}

describe('CoveragePage version comparison wiring', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listEvaluationsMock.mockReset()
    listEvaluationsMock.mockResolvedValue({ items: evaluations, total: evaluations.length })
    compareMock.mockReset()
    compareMock.mockResolvedValue(comparisonPayload())
  })

  it('offers only other finished evaluations of the same scenario as historical versions', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const values = wrapper.findAllComponents(ElOption).map((option) => option.props('value'))
    expect(values).toContain(1) // same-scenario finished evaluation is selectable
    expect(values).not.toContain(2) // current evaluation itself is excluded
    expect(values).not.toContain(3) // running evaluation is excluded
    expect(values).not.toContain(4) // other-scenario evaluation is excluded
  })

  it('requests historical -> current and renders score delta, safeguard field changes and path changes', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await chooseHistorical(wrapper, 1)

    expect(compareMock).toHaveBeenCalledTimes(1)
    expect(compareMock).toHaveBeenCalledWith(1, 2) // historical id first, current selected second

    const text = wrapper.find('.evaluation-comparison').text()
    expect(text).toContain('历史版本 #1')
    expect(text).toContain('当前版本 #2')
    expect(text).toContain('80 → 45')
    expect(text).toContain('-35')
    expect(text).toContain('字段变化')
    expect(text).toContain('有效性')
    expect(text).toContain('0.8000')
    expect(text).toContain('0.4500')
    expect(text).toContain('active')
    expect(text).toContain('invalid')
    expect(text).toContain('Old alarm')
    expect(text).toContain('移除')
    expect(wrapper.findAll('.path-change.added')).toHaveLength(4)
    expect(wrapper.findAll('.path-change.eliminated')).toHaveLength(0)
    expect(text).toContain('保护层失效或减弱')
  })

  it('resets the historical choice and clears the diff when switching the current evaluation', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await chooseHistorical(wrapper, 1)
    expect(compareMock).toHaveBeenLastCalledWith(1, 2)

    const store = useCoverageEvaluationStore()
    expect(store.comparison).toBeDefined()

    // select a different current evaluation (#1) from the history strip;
    // match the exact id badge so "#1" does not accidentally match "#11"-style labels
    const buttons = wrapper.findAll('.run-selector button')
    const historicalButton = buttons.find((button) => button.find('span').text() === '#1')
    expect(historicalButton).toBeTruthy()
    await historicalButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(compareMock).not.toHaveBeenCalledWith(1, 1) // never compares an evaluation against itself
    expect(store.comparison).toBeUndefined()
    expect(wrapper.find('.evaluation-comparison').text().trim()).toBe('')
  })

  it('clears the diff when the historical selection is emptied', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await chooseHistorical(wrapper, 1)
    expect(wrapper.find('.evaluation-comparison').text()).toContain('历史版本 #1')

    await chooseHistorical(wrapper, undefined)
    const store = useCoverageEvaluationStore()
    expect(store.comparison).toBeUndefined()
    expect(wrapper.find('.evaluation-comparison').text().trim()).toBe('')
  })

  it('shows an inline error when the compare API fails on a cross-scenario request', async () => {
    compareMock.mockRejectedValueOnce(new ApiError(
      422, 'VALIDATION_FAILED',
      'only evaluations of the same deviation scenario can be compared', null, 'req-cross',
    ))
    const wrapper = mountPage()
    await flushPromises()
    await chooseHistorical(wrapper, 1)

    const panel = wrapper.find('.evaluation-comparison')
    expect(panel.text()).toContain('无法生成版本差异')
    expect(panel.text()).toContain('same deviation scenario')
    expect(compareMock).toHaveBeenCalledWith(1, 2)
  })

  it('blocks a self comparison through the store guard without calling the API', async () => {
    // The dropdown hides the current version, but the guard must still stop a same-id pick
    // from reaching the backend (server also returns 400, covered by backend tests).
    const wrapper = mountPage()
    await flushPromises()
    const store = useCoverageEvaluationStore()

    const result = await store.compare(2, 2)
    await flushPromises()

    expect(result).toBeUndefined()
    expect(compareMock).not.toHaveBeenCalled()
    const panel = wrapper.find('.evaluation-comparison')
    expect(panel.text()).toContain('无法生成版本差异')
    expect(panel.text()).toContain('请选择与当前版本不同的历史评估')
    expect(store.comparisonError).toContain('不同')
  })
})
