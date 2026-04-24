export type ContractEditorTabId = 'details' | 'content' | 'semantic' | 'clauses' | 'builder'

interface ContractEditorUiState {
  activeTab: ContractEditorTabId
  tabs: [
    { id: 'details', label: string },
    { id: 'content', label: string },
    { id: 'semantic', label: string },
    { id: 'clauses', label: string },
    { id: 'builder', label: string },
  ]
}

export type { ContractEditorUiState }
