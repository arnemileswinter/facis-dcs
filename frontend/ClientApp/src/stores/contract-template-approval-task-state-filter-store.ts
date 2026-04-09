import type { FilterStore } from '@/models/stores/filter-store'
import type { ApprovalTaskState } from '@/types/approval-task-state'
import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'

export const useContractTemplateApprovalTaskStateFilterStore = defineStore(
  'contractTemplateApprovalTaskStateFilter',
  () => {
    const stateFilters: Ref<Set<ApprovalTaskState>> = ref(new Set())

    const hasFilters = computed(() => stateFilters.value.size > 0)

    function hasFilter(filter: ApprovalTaskState) {
      return stateFilters.value.has(filter)
    }

    function setFilter(filter: ApprovalTaskState) {
      stateFilters.value.add(filter)
    }

    function removeFilter(filter: ApprovalTaskState) {
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
    } satisfies FilterStore<ApprovalTaskState>
  },
)
