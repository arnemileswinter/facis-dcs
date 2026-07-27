<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import MachineCredentialDialog from '@/components/admin/MachineCredentialDialog.vue'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import type { ContractTarget, MachineCredential } from '@/models/responses/contract-response'

/**
 * Administration of the Contract Target Systems deployments may be dispatched
 * to (ADR-25, UC-09-01 system configuration). Deployment used to address a
 * single endpoint from process configuration, so an operator could not say
 * where a contract should go and a failed dispatch had no target to name.
 */

const targets = ref<ContractTarget[]>([])
const loading = ref(false)
const error = ref('')
const saving = ref(false)

const issued = ref<MachineCredential | null>(null)
const issuedTitle = ref('')

const editingId = ref<string | null>(null)
const form = ref({ name: '', url: '', description: '', enabled: true })

const isEditing = computed(() => editingId.value !== null)

const load = async () => {
  loading.value = true
  error.value = ''
  try {
    targets.value = await contractWorkflowService.listTargets()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load target systems'
  } finally {
    loading.value = false
  }
}

onMounted(load)

const resetForm = () => {
  editingId.value = null
  form.value = { name: '', url: '', description: '', enabled: true }
}

const edit = (target: ContractTarget) => {
  editingId.value = target.id
  form.value = {
    name: target.name,
    url: target.url,
    description: target.description ?? '',
    enabled: target.enabled,
  }
}

const save = async () => {
  saving.value = true
  error.value = ''
  try {
    const payload = {
      name: form.value.name.trim(),
      url: form.value.url.trim(),
      description: form.value.description.trim() || undefined,
      enabled: form.value.enabled,
    }
    if (editingId.value) {
      await contractWorkflowService.updateTarget({ ...payload, id: editingId.value })
    } else {
      await contractWorkflowService.createTarget(payload)
    }
    resetForm()
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not save the target system'
  } finally {
    saving.value = false
  }
}

// A target proves it is itself when it acknowledges a deployment, so the
// credential is issued per target rather than shared (ADR-27). Issuing again
// stops the previous secret working.
const issueCredential = async (target: ContractTarget) => {
  error.value = ''
  try {
    const credential = await contractWorkflowService.rotateTargetSecret(target.id)
    issuedTitle.value = `Callback credential for ${target.name}`
    issued.value = credential
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not issue the callback credential'
  }
}

// Removal is refused by the backend while a contract still designates the
// target, so the message it returns is shown rather than swallowed: it names
// how many contracts must be repointed first.
const remove = async (target: ContractTarget) => {
  error.value = ''
  try {
    await contractWorkflowService.deleteTarget(target.id)
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not remove the target system'
  }
}
</script>

<template>
  <div data-testid="target-admin" class="flex flex-col gap-6 p-6">
    <div>
      <h1 class="text-2xl font-semibold">Target Systems</h1>
      <p class="mt-1 opacity-70">
        Systems a signed contract can be deployed to. Each contract names the one it deploys to, so the deployment that
        follows signing has a destination.
      </p>
    </div>

    <div v-if="error" data-testid="target-admin-error" class="alert rounded-box alert-error">{{ error }}</div>

    <section class="card bg-base-200">
      <form class="card-body grid gap-3 md:grid-cols-2" @submit.prevent="save">
        <h2 class="card-title md:col-span-2">{{ isEditing ? 'Change target system' : 'Register a target system' }}</h2>

        <label class="form-control">
          <span class="label-text">Name</span>
          <input v-model="form.name" data-testid="target-name" required class="input-bordered input" />
        </label>

        <label class="form-control">
          <span class="label-text">Endpoint URL</span>
          <input v-model="form.url" data-testid="target-url" required type="url" class="input-bordered input" />
        </label>

        <label class="form-control md:col-span-2">
          <span class="label-text">Description</span>
          <input v-model="form.description" data-testid="target-description" class="input-bordered input" />
        </label>

        <label class="flex cursor-pointer items-center gap-2">
          <input v-model="form.enabled" data-testid="target-enabled" type="checkbox" class="toggle toggle-sm" />
          <span class="label-text">Accepts deployments</span>
        </label>

        <div class="flex gap-2 md:col-span-2">
          <button type="submit" data-testid="target-save" :disabled="saving" class="btn btn-primary">
            {{ saving ? 'Saving…' : isEditing ? 'Save changes' : 'Register' }}
          </button>
          <button v-if="isEditing" type="button" data-testid="target-cancel" class="btn btn-ghost" @click="resetForm">
            Cancel
          </button>
        </div>
      </form>
    </section>

    <section>
      <div v-if="loading" class="opacity-70">Loading…</div>
      <div v-else-if="targets.length === 0" data-testid="target-empty-state" class="alert rounded-box alert-info">
        No target system is registered yet. A contract cannot be deployed until one exists and it names it.
      </div>
      <div v-else class="overflow-x-auto">
        <table class="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Endpoint</th>
              <th>Description</th>
              <th>Deployments</th>
              <th>Callback credential</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="target in targets" :key="target.id" data-testid="target-row">
              <td data-testid="target-row-name">{{ target.name }}</td>
              <td class="font-mono text-xs">{{ target.url }}</td>
              <td>{{ target.description }}</td>
              <td>
                <span v-if="target.enabled" class="badge badge-sm badge-success">accepted</span>
                <span v-else data-testid="target-row-disabled" class="badge badge-sm badge-warning">refused</span>
              </td>
              <td class="text-xs">
                <span
                  v-if="target.oauth_client_id"
                  data-testid="target-row-credentialed"
                  class="badge badge-sm badge-success"
                >
                  issued
                </span>
                <span v-else data-testid="target-row-no-credential" class="badge badge-ghost badge-sm">none</span>
              </td>
              <td class="flex gap-2">
                <button class="btn btn-ghost btn-xs" data-testid="target-edit" @click="edit(target)">Edit</button>
                <button
                  class="btn btn-ghost btn-xs"
                  data-testid="target-issue-credential"
                  @click="issueCredential(target)"
                >
                  {{ target.oauth_client_id ? 'New secret' : 'Issue credential' }}
                </button>
                <button class="btn text-error btn-ghost btn-xs" data-testid="target-delete" @click="remove(target)">
                  Remove
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <MachineCredentialDialog v-if="issued" :credential="issued" :title="issuedTitle" @close="issued = null" />
  </div>
</template>
