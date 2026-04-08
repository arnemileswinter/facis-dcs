import { defineStore } from 'pinia'
import type { ContractEditorTabId, ContractEditorUiState } from '../models/contract-editor-ui-store'

const storeId = 'contractEditorUi'
const defaultState: Readonly<ContractEditorUiState> = {
  activeTab: 'details',
  tabs: [
    { id: 'details', label: 'Contract Details' },
    { id: 'content', label: 'Contract Content' },
  ],
}

export const useContractEditorUiStore = defineStore(storeId, {
  state: (): ContractEditorUiState => getInitialState(),
  actions: {
    setActiveTab(tab: ContractEditorTabId) {
      this.activeTab = tab
    },
    reset(overrides?: Partial<ContractEditorUiState>) {
      Object.assign(this, getInitialState())
      if (overrides) Object.assign(this, overrides)
    },
  },
})

function getInitialState(): ContractEditorUiState {
  return {
    ...defaultState,
    tabs: [...defaultState.tabs],
  }
}
