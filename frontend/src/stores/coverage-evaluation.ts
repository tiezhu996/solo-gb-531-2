import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import * as coverageApi from '../api/coverage-evaluation'
import type { CoverageEvaluation, EvaluationComparison } from '../types/coverage-evaluation'

export const useCoverageEvaluationStore = defineStore('coverage-evaluations', () => {
  const items = ref<CoverageEvaluation[]>([])
  const selectedId = ref<number>()
  const loading = ref(false)
  const comparison = ref<EvaluationComparison>()
  const comparisonLoading = ref(false)
  const comparisonError = ref('')
  const selected = computed(() => items.value.find((item) => item.id === selectedId.value))
  async function load(scenarioId?: number) { loading.value = true; try { items.value = (await coverageApi.listCoverageEvaluations(scenarioId)).items; if (!items.value.some((x) => x.id === selectedId.value)) selectedId.value = items.value[0]?.id } finally { loading.value = false } }
  async function run(scenarioId: number, key: string) { const item = await coverageApi.runCoverageEvaluation({ scenario_id: scenarioId }, key); await load(); selectedId.value = item.id; return item }
  async function refresh(id: number) { const item = await coverageApi.getCoverageEvaluation(id); const index = items.value.findIndex((x) => x.id === id); if (index >= 0) items.value[index] = item; else items.value.unshift(item); return item }
  async function confirm(id: number) { const item = await coverageApi.confirmCoverageEvaluation(id); await load(); selectedId.value = item.id; return item }
  async function voidRun(id: number) { const item = await coverageApi.voidCoverageEvaluation(id); await load(); selectedId.value = item.id; return item }
  async function replay(id: number) { return coverageApi.replayCoverageEvaluation(id) }
  async function compare(baseId: number, comparedId: number) {
    if (baseId === comparedId) {
      comparison.value = undefined
      comparisonError.value = '请选择与当前版本不同的历史评估'
      return undefined
    }
    comparisonLoading.value = true
    comparisonError.value = ''
    try {
      comparison.value = await coverageApi.compareCoverageEvaluations(baseId, comparedId)
      return comparison.value
    } catch (error) {
      comparison.value = undefined
      throw error
    } finally {
      comparisonLoading.value = false
    }
  }
  function clearComparison() { comparison.value = undefined; comparisonError.value = '' }
  return { items, selectedId, selected, loading, comparison, comparisonLoading, comparisonError, load, run, refresh, confirm, voidRun, replay, compare, clearComparison }
})
