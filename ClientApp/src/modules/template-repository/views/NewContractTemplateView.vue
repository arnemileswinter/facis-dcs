<template>
    <div class="flex flex-col min-h-full -mx-4 md:-mx-8 -my-4 md:-my-8">
        <!-- Create flow: show only type selection until user chooses -->
        <div v-if="showTypeSelectionOnly" class="max-w-4xl mx-auto px-6 py-12 flex flex-col gap-6">
            <h1 class="text-2xl font-bold text-base-content">Choose contract type</h1>
            <TemplateTypeSelect
                :model-value="templateType"
                @update:model-value="onTemplateTypeChosen($event)"
            />
            <div class="flex justify-end pt-4">
                <button type="button" class="btn btn-ghost" @click="router.back()">Cancel</button>
            </div>
        </div>
        <template v-else>
        <div class="sticky top-0 z-10 shrink-0 bg-base-200 border-b border-base-300">
            <div class="max-w-4xl mx-auto px-6 pt-3">
                <p class="text-xs font-black uppercase tracking-widest text-base-content/40 mb-2">
                    {{ isEditMode ? 'Edit Template' : 'New Template' }}
                </p>
                <div role="tablist" class="tabs tabs-lift tabs-lg">
                    <a v-for="(tab, _index) in tabs" :key="tab.id" role="tab" class="tab"
                        :class="{ 'tab-active': activeTab === tab.id }" @click="setActiveTab(tab.id)">
                        {{ tab.label }}
                    </a>
                </div>
            </div>
        </div>

        <!-- Tab content -->
        <div class="grow mt-5">
            <div class="max-w-4xl mx-auto p-6">
                <div class="grid grid-cols-1 gap-4">

                    <!-- DETAILS TAB -->
                    <div v-show="activeTab === 'details'">
                        <div class="card bg-base-100 border border-base-300 shadow-sm">
                            <div class="card-body gap-5">
                                <h2 class="card-title text-sm">
                                    <span class="badge badge-primary">01</span> Template Details
                                </h2>
                                <DetailsEditor />
                            </div>
                        </div>
                    </div>

                    <!-- SEMANTIC RULES TAB -->
                    <div v-show="activeTab === 'semantic'">
                        <div class="card bg-base-100 border border-base-300 shadow-sm">
                            <div class="card-body gap-5">
                                <h2 class="card-title text-sm">
                                    <span class="badge badge-secondary">02</span> Semantic Rules
                                </h2>
                                <SemanticRulesEditor />
                            </div>
                        </div>

                    </div>

                    <!-- CLAUSES TAB -->
                    <div v-show="activeTab === 'clauses'">
                        <div class="card bg-base-100 border border-base-300 shadow-sm">
                            <div class="card-body gap-5">
                                <h2 class="card-title text-sm">
                                    <span class="badge badge-primary">03</span> Clauses
                                </h2>
                                <ClausesEditor />
                            </div>
                        </div>
                    </div>

                    <!-- BUILDER TAB -->
                    <div v-show="activeTab === 'builder'">
                        <div class="card bg-base-100 border border-base-300 shadow-sm">
                            <div class="card-body">
                                <div class="flex items-center justify-between mb-2">
                                    <h2 class="card-title text-sm">Builder</h2>
                                    <button
                                      type="button"
                                      class="btn btn-sm btn-secondary"
                                      @click="togglePreviewDialog"
                                    >
                                      Preview
                                    </button>
                                </div>
                                <BuilderEditor />
                            </div>
                        </div>
                        <AddBlockModal />
                        <BuilderPreviewDialog />
                    </div>

                    <!-- META TAB -->
                    <div v-show="activeTab === 'meta'">
                        <div class="card bg-base-100 border border-base-300 shadow-sm">
                            <div class="card-body">
                                <h2 class="card-title text-sm">Meta Data</h2>
                                <MetaDataEditor />
                            </div>
                        </div>
                    </div>

                </div>
            </div>
        </div>

        <!-- Pinned Footer -->
        <div v-if="draftStore.isEditable" class="sticky bottom-0 shrink-0 border-t border-base-300 bg-base-100">
            <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
                <button class="btn btn-ghost md:w-32" @click="router.back()">Cancel</button>
                <button @click="submit" class="btn btn-primary flex-1" :disabled="isSubmitting">
                    <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
                    {{ isEditMode ? 'Update Template' : 'Create' }}
                </button>
            </div>
        </div>
        </template>
    </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useTemplateEditorUiStore } from '@template-repository/store/templateEditorUiStore.ts'
import { useTemplateDraftStore } from '@template-repository/store/templateDraftStore'
import BuilderEditor from '@template-repository/components/BuilderEditor.vue'
import AddBlockModal from '@template-repository/components/builder-editor/AddBlockModal.vue'
import SemanticRulesEditor from '@template-repository/components/SemanticRulesEditor.vue'
import ClausesEditor from '@template-repository/components/ClausesEditor.vue'
import DetailsEditor from '@template-repository/components/DetailsEditor.vue'
import MetaDataEditor from '@template-repository/components/MetaDataEditor.vue'
import BuilderPreviewDialog from '@template-repository/components/builder-editor/BuilderPreviewDialog.vue'
import TemplateTypeSelect from '@template-repository/components/TemplateTypeSelect.vue'
import { storeToRefs } from 'pinia'
import { ContractTemplateService } from '@/services/contract-template-service'
import { useToNumber } from '@vueuse/core'
import { useApprovedSubTemplateStore } from '@template-repository/store/approvedSubTemplateStore'
import { isApprovedTemplateBlock } from '@template-repository/models/contract-templace'

const router = useRouter()
const route = useRoute()

const templateEditorUiStore = useTemplateEditorUiStore()
const approvedSubTemplateStore = useApprovedSubTemplateStore()
const draftStore = useTemplateDraftStore()
const { activeTab } = storeToRefs(templateEditorUiStore)
const { templateType } = storeToRefs(draftStore)
const { setActiveTab, togglePreviewDialog } = templateEditorUiStore

const isEditMode = computed(() => !!route.params.did)
const hasChosenType = ref(false)
const showTypeSelectionOnly = computed(() => !isEditMode.value && !hasChosenType.value)
const tabs = computed(() => templateEditorUiStore.availableTabs(templateType.value))

function onTemplateTypeChosen(value: typeof templateType.value) {
    draftStore.reset({ templateType: value })
    hasChosenType.value = true
}

watch(isEditMode, (isEdit) => {
    approvedSubTemplateStore.resetTemplates()
    templateEditorUiStore.reset()
    if (isEdit) {
        hasChosenType.value = true
        // load template data into draftStore
        const did = `${route.params.did}`
        const version = useToNumber(`${route.query.version}`).value
        const document_number = useToNumber(`${route.query.document_number}`).value
        ContractTemplateService.retrieveById({ did, version, document_number })
            .then(async template => {
                if (!template) {
                    draftStore.reset()
                    return
                }

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
                    version: template.version,
                    document_number: template.document_number
                })

                const approvedBlocks = draftStore.documentBlocks.filter((b) => isApprovedTemplateBlock(b))

                for (const block of approvedBlocks) {
                    const template = await ContractTemplateService.retrieveById({
                        did: block.templateId,
                        version: block.version,
                        document_number: block.document_number,
                    })
                    if (template) {
                        approvedSubTemplateStore.addTemplate(template)
                    }
                }
            })
            .catch(error => {
                console.error('Failed to load template for editing', error)
            })
        
    }
    else { draftStore.reset(); hasChosenType.value = false }
}, { immediate: true })

const isSubmitting = ref(false)

const submit = async () => {
    isSubmitting.value = true
    try {
        if (!draftStore.hasTemplateId) {
            const data = draftStore.templateCreateRequestData
            await ContractTemplateService.create(data)
        } else {
            const data = draftStore.templateUpdateRequestData
            if (data) {
                await ContractTemplateService.update(data)
            }
        }
        router.push({ name: 'templates.list' })
    } catch (error) {
        console.error('Submission failed', error)
    } finally {
        isSubmitting.value = false
    }
}
</script>
