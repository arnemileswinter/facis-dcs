<template>
  <div class="flex flex-col min-h-full -mx-4 md:-mx-8 -my-4 md:-my-8">
    <TemplateEditors title="View Template" />

    <!-- Pinned Footer -->
    <div class="sticky bottom-0 shrink-0 border-t border-base-300 bg-base-100">
      <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
        <button class="btn btn-ghost md:w-32" @click="router.back()">Back</button>
        <TemplateManagerActions v-if="contractTemplate && isManager" :item="contractTemplate" class="btn btn-primary flex-1" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import TemplateManagerActions from '@/components/lists/template/template-list/TemplateManagerActions.vue'
import type { PartialContractTemplate } from '@/models/contract-template'
import { contractTemplateService } from '@/services/contract-template-service'
import { useAuthStore } from '@/stores/auth-store'
import TemplateEditors from '@template-repository/components/TemplateEditors.vue'
import { isApprovedTemplateBlock } from '@template-repository/models/contract-templace'
import { useApprovedSubTemplateStore } from '@template-repository/store/approvedSubTemplateStore'
import { useTemplateDraftStore } from '@template-repository/store/templateDraftStore'
import { useTemplateEditorUiStore } from '@template-repository/store/templateEditorUiStore.ts'
import { computed, ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const router = useRouter()
const route = useRoute()

const authStore = useAuthStore()
const templateEditorUiStore = useTemplateEditorUiStore()
const approvedSubTemplateStore = useApprovedSubTemplateStore()
const draftStore = useTemplateDraftStore()

const hasDid = computed(() => !!route.params.did)
const hasChosenType = ref(false)

const isManager = computed(() => {
  return hasDid.value && (authStore.user?.roles?.includes('TEMPLATE_MANAGER') ?? false)
})

const contractTemplate: Ref<PartialContractTemplate | null> = ref(null)

watch(hasDid, (hasDid) => {
  approvedSubTemplateStore.resetTemplates()
  templateEditorUiStore.reset()
  if (!hasDid) return

  hasChosenType.value = true
  const did = `${route.params.did}`
  contractTemplateService.retrieveById({ did })
    .then(async template => {
      if (!template) {
        draftStore.reset()
        return
      }
      templateEditorUiStore.setTemplateEditable(false)
      contractTemplate.value = template

      draftStore.reset({
        did: template.did,
        name: template.name,
        description: template.description,
        documentOutline: template.template_data?.documentOutline ?? [],
        documentBlocks: template.template_data?.documentBlocks ?? [],
        semanticConditions: template.template_data?.semanticConditions ?? [],
        customMetaData: template.template_data?.customMetaData ?? [],
        templateType: template.template_type,
        state: template.state,
        version: template.version ?? null,
        document_number: template.document_number ?? null,
        updated_at: template.updated_at ?? null,
      })

      const approvedBlocks = draftStore.documentBlocks.filter((b) => isApprovedTemplateBlock(b))

      for (const block of approvedBlocks) {
        const template = await contractTemplateService.retrieveById({ did: block.templateId })
        if (template) {
          approvedSubTemplateStore.addTemplate(template)
        }
      }
    })
    .catch(error => {
      console.error('Failed to load template for editing', error)
    })

}, { immediate: true })

</script>
