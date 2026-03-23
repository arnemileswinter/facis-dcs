import { defineStore } from 'pinia'
import type { ContractTemplate } from '@/models/contract-template'
import { TemplateState } from '@/types/contract-template-state'

const storeId = "approveSubTemplate"
const defaultState = {
  templates: [] as ContractTemplate[],
}

export const useApprovedSubTemplateStore = defineStore(storeId, {
  state: () => getInitialState(),
  getters: {},
  actions: {
    addTemplate(template: ContractTemplate) {
      if (template.state !== TemplateState.approved) return
      const newTemplates = this.templates.filter(t => !isSameTemplate(t, template))
      newTemplates.push(template)
      this.templates = newTemplates
    },
    removeTemplate(template: { did: string, version: number, document_number: number }) {
      this.templates = this.templates.filter(t => !isSameTemplate(t, template))
    },
    resetTemplates() {
      Object.assign(this, { ...getInitialState() })
    }
  }
})

const isSameTemplate = (t1: { did: string, version: number, document_number: number }, t2: { did: string, version: number, document_number: number }) => {
  return t1.did === t2.did && t1.version === t2.version && t1.document_number === t2.document_number
}

function getInitialState() {
  return {
    ...defaultState
  }
}