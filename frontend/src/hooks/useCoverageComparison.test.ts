import { nextTick, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('element-plus', () => ({ ElMessage: { error: vi.fn(), success: vi.fn(), warning: vi.fn() } }))

const compareMock = vi.fn()
const clearComparisonMock = vi.fn()
vi.mock('../stores/coverage-evaluation', () => ({
  useCoverageEvaluationStore: () => ({
    compare: compareMock,
    clearComparison: clearComparisonMock,
    comparison: ref(undefined),
    comparisonLoading: ref(false),
    comparisonError: ref(''),
  }),
}))

import { ElMessage } from 'element-plus'
import { useCoverageComparison } from './useCoverageComparison'
import type { CoverageEvaluation } from '../types/coverage-evaluation'

function evaluation(id: number): CoverageEvaluation {
  return {
    id,
    scenario_id: 1,
    algorithm_version: 'hazop-cover-v1.0.0',
    input_snapshot: '',
    coverage_score: 50,
    uncovered_paths: [],
    risk_rank_before: 'high',
    risk_rank_after: 'medium',
    evaluation_state: 'completed',
    explanation: '',
    evaluated_by: 1,
    evaluated_at: '2026-09-01T00:00:00Z',
  }
}

describe('useCoverageComparison direction', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    compareMock.mockReset()
    clearComparisonMock.mockReset()
    vi.mocked(ElMessage.error).mockClear()
  })

  it('calls compare as historical -> current so deltas read from the selected history to the open version', async () => {
    const current = ref(evaluation(5))
    const hook = useCoverageComparison(current)

    hook.historicalId.value = 2
    await nextTick()
    await Promise.resolve()

    expect(compareMock).toHaveBeenCalledTimes(1)
    expect(compareMock).toHaveBeenCalledWith(2, 5)
  })

  it('clears the historical selection when the current evaluation changes', async () => {
    const current = ref(evaluation(5))
    const hook = useCoverageComparison(current)
    hook.historicalId.value = 2
    await nextTick()
    await Promise.resolve()
    expect(compareMock).toHaveBeenLastCalledWith(2, 5)

    current.value = evaluation(6)
    await nextTick()
    expect(hook.historicalId.value).toBeUndefined()
  })

  it('clears comparison when the dropdown is emptied', async () => {
    const current = ref(evaluation(5))
    const hook = useCoverageComparison(current)
    hook.historicalId.value = 2
    await nextTick()
    await Promise.resolve()
    hook.historicalId.value = undefined
    await nextTick()
    expect(clearComparisonMock).toHaveBeenCalled()
  })

  it('surfaces backend errors inline without reversing or hiding the failure', async () => {
    compareMock.mockRejectedValueOnce(new Error('only evaluations of the same deviation scenario can be compared'))
    const current = ref(evaluation(5))
    const hook = useCoverageComparison(current)
    hook.historicalId.value = 9
    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(hook.inlineError.value).toContain('same deviation scenario')
    expect(ElMessage.error).toHaveBeenCalled()
  })
})
