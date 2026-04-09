import type { FilterStore } from '@/models/stores/filter-store'
import type { ReviewTaskState } from '@/types/review-task-state'
import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'

export const useContractTemplateReviewTaskStateFilterStore = defineStore(
  'contractTemplateReviewTaskStateFilter',
  () => {
    const stateFilters: Ref<Set<ReviewTaskState>> = ref(new Set())

    const hasFilters = computed(() => stateFilters.value.size > 0)

    function hasFilter(filter: ReviewTaskState) {
      return stateFilters.value.has(filter)
    }

    function setFilter(filter: ReviewTaskState) {
      stateFilters.value.add(filter)
    }

    function removeFilter(filter: ReviewTaskState) {
      stateFilters.value.delete(filter)
    }

    function reset() {
      stateFilters.value.clear()
    }

    return {
      stateFilters,
      hasFilters,
      hasFilter,
      setFilter,
      removeFilter,
      reset,
    } satisfies FilterStore<ReviewTaskState>
  },
)
