export type ContractEditorTabId = 'details' | 'content'

interface ContractEditorUiState {
  activeTab: ContractEditorTabId
  tabs: [
    { id: 'details', label: string },
    { id: 'content', label: string },
  ]
}

export type { ContractEditorUiState }
