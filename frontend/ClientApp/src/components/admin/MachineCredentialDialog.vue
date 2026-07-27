<script setup lang="ts">
import { ref } from 'vue'
import type { MachineCredential } from '@/models/responses/contract-response'

/**
 * Shows an issued machine credential (ADR-27). The secret is in the response
 * that opened this dialog and in no other: Hydra keeps only a hash, so closing
 * without copying means rotating, not recovering. The wording says so rather
 * than leaving the operator to discover it.
 */

defineProps<{ credential: MachineCredential; title: string }>()
const emit = defineEmits<{ close: [] }>()

const copied = ref(false)

const copy = async (value: string) => {
  await navigator.clipboard.writeText(value)
  copied.value = true
}
</script>

<template>
  <div class="modal-open modal" data-testid="credential-dialog">
    <div class="modal-box max-w-2xl">
      <h3 class="text-lg font-semibold">{{ title }}</h3>

      <div class="mt-4 alert alert-warning">
        <span data-testid="credential-once-warning">
          This secret is shown once. It is not stored and cannot be retrieved — if it is lost, issue a new one, which
          stops this one working.
        </span>
      </div>

      <div class="mt-4 flex flex-col gap-3">
        <label class="form-control">
          <span class="label-text">Client ID</span>
          <input
            :value="credential.client_id"
            data-testid="credential-client-id"
            readonly
            class="input-bordered input font-mono text-sm"
          />
        </label>

        <label class="form-control">
          <span class="label-text">Client secret</span>
          <div class="join">
            <input
              :value="credential.client_secret"
              data-testid="credential-secret"
              readonly
              class="input-bordered input join-item w-full font-mono text-sm"
            />
            <button
              type="button"
              class="btn join-item"
              data-testid="credential-copy"
              @click="copy(credential.client_secret)"
            >
              {{ copied ? 'Copied' : 'Copy' }}
            </button>
          </div>
        </label>

        <label v-if="credential.token_url" class="form-control">
          <span class="label-text">Token endpoint</span>
          <input
            :value="credential.token_url"
            data-testid="credential-token-url"
            readonly
            class="input-bordered input font-mono text-sm"
          />
          <span class="label-text-alt mt-1 opacity-70">
            Present the credential here with grant_type=client_credentials.
          </span>
        </label>
      </div>

      <div class="modal-action">
        <button type="button" class="btn btn-primary" data-testid="credential-done" @click="emit('close')">
          I have copied it
        </button>
      </div>
    </div>
  </div>
</template>
