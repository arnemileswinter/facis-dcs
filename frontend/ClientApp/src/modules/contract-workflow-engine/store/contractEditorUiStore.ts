import { defineStore } from 'pinia'
import type { ContractEditorTabId, ContractEditorUiState } from '../models/contract-editor-ui-store'
import type { ContractState as ContractStateType } from '@/types/contract-state'
import { ContractState } from '@/types/contract-state'

const storeId = 'contractEditorUi'
const defaultState: Readonly<ContractEditorUiState> = {
  activeTab: 'details',
  tabs: [
    { id: 'details', label: 'Contract Details' },
    { id: 'content', label: 'Contract Content' },
    { id: 'semantic', label: 'Semantic Rules' },
    { id: 'clauses', label: 'Clauses' },
    { id: 'builder', label: 'Builder' },
  ],
}

export const useContractEditorUiStore = defineStore(storeId, {
  state: (): ContractEditorUiState => getInitialState(),
  actions: {
    setActiveTab(tab: ContractEditorTabId) {
      this.activeTab = tab
    },
    availableTabs(contractState: ContractStateType) {
      switch (contractState) {
        case ContractState.draft:
          // Keep the edit page simple for now, same for the negotiation states
          return this.tabs.filter(tab => ['details', 'content'].includes(tab.id))
          default:
          return this.tabs.filter(tab => ['details', 'content'].includes(tab.id))
      }
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
