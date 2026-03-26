<template>
  <div class="flex flex-col min-h-full -mx-4 md:-mx-8 -my-4 md:-my-8">

    <TemplateEditors title="Approve Template" />

    <!-- Pinned Footer -->
    <div v-if="hasDid" class="sticky bottom-0 shrink-0 border-t border-base-300 bg-base-100">
      <!-- Decision notes container -->
      <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
        <textarea v-model="decisionNote" :disabled="isSubmitting"
          class="textarea textarea-ghost textarea-sm w-full mt-0.5 text-sm min-h-10 resize-y border border-base-300/50 rounded-lg"
          placeholder="Decision Note" rows="4" />
      </div>
      <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
        <button class="btn btn-ghost md:w-32" @click="router.back()">Cancel</button>
        <button @click="returnToDraft" class="btn btn-primary flex-1" :disabled="isSubmitting">
          <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
          Return to draft
        </button>
        <button @click="reopenReviews" class="btn btn-primary flex-1" :disabled="isSubmitting">
          <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
          Reopen review
        </button>
        <button @click="approve" class="btn btn-primary flex-1" :disabled="isSubmitting">
          <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
          Approve
        </button>
        <TemplateManagerActions v-if="contractTemplate && isManager" :item="contractTemplate" class="btn btn-primary flex-1" />
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, type Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import TemplateManagerActions from '@/components/lists/template-list/TemplateManagerActions.vue'
import type { PartialContractTemplate } from '@/models/contract-template'
import { ROUTES } from '@/router/router'
import { useAuthStore } from '@/stores/auth-store'
import { useTemplateEditorUiStore } from '@template-repository/store/templateEditorUiStore.ts'
import { useTemplateDraftStore } from '@template-repository/store/templateDraftStore'
import TemplateEditors from '@template-repository/components/TemplateEditors.vue'
import { contractTemplateService } from '@/services/contract-template-service'
import { useApprovedSubTemplateStore } from '@template-repository/store/approvedSubTemplateStore'
import { isApprovedTemplateBlock } from '@template-repository/models/contract-templace'

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

const isSubmitting = ref(false)
const decisionNote = ref<string>('')

async function approve() {
  const did = draftStore.did
  const updatedAt = draftStore.updated_at
  if (!did || !updatedAt) {
    console.error('Missing did or updated_at for approval')
    return
  }
  isSubmitting.value = true
  try {
    await contractTemplateService.approve({
      did,
      updated_at: updatedAt,
      decision_notes: decisionNote.value ? [decisionNote.value] : [],
    })
    router.push({ name: ROUTES.TEMPLATES.LIST })
  } catch (error) {
    console.error('Approval failed', error)
  } finally {
    isSubmitting.value = false
  }
}

async function reopenReviews() {
  const did = draftStore.did
  const updatedAt = draftStore.updated_at
  if (!did || !updatedAt) {
    console.error('Missing did or updated_at for reopen reviews')
    return
  }
  isSubmitting.value = true
  try {
    await contractTemplateService.submit({
      did,
      updated_at: updatedAt,
      comments: decisionNote.value ? [decisionNote.value] : []
    })
    router.push({ name: ROUTES.TEMPLATES.LIST })
  } catch (error) {
    console.error('Reopen reviews failed', error)
  } finally {
    isSubmitting.value = false
  }
}

async function returnToDraft() {
  const did = draftStore.did
  const updatedAt = draftStore.updated_at
  if (!did || !updatedAt) {
    console.error('Missing did or updated_at for rejection')
    return
  }
  if (!decisionNote.value?.trim()) {
    console.error('Reason is required for rejection')
    return
  }
  isSubmitting.value = true
  try {
    await contractTemplateService.reject({
      did,
      updated_at: updatedAt,
      reason: decisionNote.value.trim(),
    })
    router.push({ name: ROUTES.TEMPLATES.LIST })
  } catch (error) {
    console.error('Rejection failed', error)
  } finally {
    isSubmitting.value = false
  }
}
</script>
