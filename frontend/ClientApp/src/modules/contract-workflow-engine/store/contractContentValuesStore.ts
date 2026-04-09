import { defineStore } from 'pinia'
import type { ContractContentValuesState } from '../models/contract-content-values-store'
import type { SemanticConditionValue } from '@/models/contract-data'

const storeId = 'contractContentValues'
const defaultState: Readonly<ContractContentValuesState> = {
  semanticConditionValues: [],
}

export const useContractContentValuesStore = defineStore(storeId, {
  state: (): ContractContentValuesState => getInitialState(),
  actions: {
    setSemanticConditionValue(payload: SemanticConditionValue) {
      const idx = this.semanticConditionValues.findIndex(
        (item) =>
          item.blockId === payload.blockId &&
          item.subBlockId === payload.subBlockId &&
          item.conditionId === payload.conditionId &&
          item.parameterName === payload.parameterName,
      )
      if (idx >= 0) {
        this.semanticConditionValues[idx] = { ...this.semanticConditionValues[idx], ...payload }
        return
      }
      this.semanticConditionValues.push(payload)
    },
    reset(overrides?: Partial<ContractContentValuesState>) {
      Object.assign(this, getInitialState())
      if (overrides) Object.assign(this, overrides)
    },
  },
})

function getInitialState(): ContractContentValuesState {
  return {
    ...defaultState,
  }
}
