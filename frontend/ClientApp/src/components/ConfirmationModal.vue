<script setup lang="ts">
import { useConfirmDialog } from '@vueuse/core'
import { ref, useTemplateRef, watch } from 'vue'

interface ModalData {
  message: string
}

const actionModal = useTemplateRef('action-modal')
const modalData = ref<ModalData>({ message: 'Confirm selection' })

const { isRevealed, reveal, confirm, cancel, onReveal } = useConfirmDialog<ModalData>()

onReveal((data) => {
  modalData.value = data
})

watch(isRevealed, (value) => {
  value ? actionModal.value?.showModal() : actionModal.value?.close()
})

defineExpose({ reveal })
</script>

<template>
  <dialog ref="action-modal" @close="cancel" class="modal modal-bottom sm:modal-middle">
    <div class="modal-box">
      <h3 class="text-lg font-bold">Confirmation</h3>
      <p class="text-md py-4">{{ modalData.message }}</p>
      <div class="modal-action flex-col">
        <button class="btn btn-soft btn-sm btn-primary" @click="confirm">Confirm</button>
        <button class="btn btn-soft btn-sm btn-error" @click="cancel">Cancel</button>
      </div>
    </div>
    <div class="modal-backdrop" @click="cancel"></div>
  </dialog>
</template>
