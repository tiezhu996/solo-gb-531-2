import { computed, ref, watch, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { errorMessage } from '../api/client'
import { useCoverageEvaluationStore } from '../stores/coverage-evaluation'
import type { CoverageEvaluation } from '../types/coverage-evaluation'

// useCoverageComparison wires the version selector to the read-only compare API.
// Direction is fixed: the dropdown value is the HISTORICAL version (base) and
// the currently selected evaluation is the CURRENT version (compared), so every
// delta reads as "historical -> current".
export function useCoverageComparison(current: Ref<CoverageEvaluation | undefined>) {
  const evaluations = useCoverageEvaluationStore()
  const historicalId = ref<number>()
  const inlineError = ref('')
  const comparison = computed(() => evaluations.comparison)
  const loading = computed(() => evaluations.comparisonLoading)
  const storeError = computed(() => evaluations.comparisonError)

  async function load(historical?: number) {
    inlineError.value = ''
    if (!current.value || !historical) {
      evaluations.clearComparison()
      return
    }
    try {
      await evaluations.compare(historical, current.value.id)
    } catch (error) {
      inlineError.value = errorMessage(error)
      ElMessage.error(inlineError.value)
    }
  }

  watch(historicalId, (id) => { void load(id) })
  watch(current, () => {
    if (historicalId.value !== undefined) historicalId.value = undefined
    else evaluations.clearComparison()
  })

  return {
    historicalId,
    inlineError,
    comparison,
    loading,
    storeError,
    load,
  }
}
