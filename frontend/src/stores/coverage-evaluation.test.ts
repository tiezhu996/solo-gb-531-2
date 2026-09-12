import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ApiError } from '../api/client'
import * as coverageApi from '../api/coverage-evaluation'
import { useCoverageEvaluationStore } from './coverage-evaluation'
import type { EvaluationComparison } from '../types/coverage-evaluation'

vi.mock('../api/coverage-evaluation', () => ({
  listCoverageEvaluations: vi.fn(),
  getCoverageEvaluation: vi.fn(),
  runCoverageEvaluation: vi.fn(),
  confirmCoverageEvaluation: vi.fn(),
  voidCoverageEvaluation: vi.fn(),
  replayCoverageEvaluation: vi.fn(),
  compareCoverageEvaluations: vi.fn(),
}))

const comparison = { base_id: 1, compared_id: 2 } as EvaluationComparison

describe('coverage evaluation store comparison', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(coverageApi.compareCoverageEvaluations).mockReset()
  })

  it('fetches and stores a read-only comparison', async () => {
    vi.mocked(coverageApi.compareCoverageEvaluations).mockResolvedValue(comparison)
    const store = useCoverageEvaluationStore()
    const result = await store.compare(1, 2)
    expect(result).toEqual(comparison)
    expect(store.comparison).toEqual(comparison)
    expect(store.comparisonLoading).toBe(false)
    expect(store.comparisonError).toBe('')
    expect(coverageApi.compareCoverageEvaluations).toHaveBeenCalledWith(1, 2)
    expect(coverageApi.runCoverageEvaluation).not.toHaveBeenCalled()
  })

  it('rejects comparing an evaluation against itself without calling the API', async () => {
    const store = useCoverageEvaluationStore()
    const result = await store.compare(3, 3)
    expect(result).toBeUndefined()
    expect(store.comparisonError).toContain('不同')
    expect(coverageApi.compareCoverageEvaluations).not.toHaveBeenCalled()
  })

  it('surfaces backend errors (different scenario / unfinished / corrupted snapshot)', async () => {
    const apiError = new ApiError(422, 'VALIDATION_FAILED', 'only evaluations of the same deviation scenario can be compared', null, 'req-1')
    vi.mocked(coverageApi.compareCoverageEvaluations).mockRejectedValue(apiError)
    const store = useCoverageEvaluationStore()
    store.comparison = comparison
    await expect(store.compare(1, 9)).rejects.toBe(apiError)
    expect(store.comparison).toBeUndefined()
    expect(store.comparisonLoading).toBe(false)
  })

  it('clears comparison state', async () => {
    vi.mocked(coverageApi.compareCoverageEvaluations).mockResolvedValue(comparison)
    const store = useCoverageEvaluationStore()
    await store.compare(1, 2)
    store.clearComparison()
    expect(store.comparison).toBeUndefined()
    expect(store.comparisonError).toBe('')
  })
})
