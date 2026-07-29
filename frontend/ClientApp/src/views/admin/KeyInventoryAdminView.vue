<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { type HSMKeyInfo, keyInventoryService } from '@/services/key-inventory-service'

/**
 * Read-only inventory of the HSM-held keys (DCS-NFR-SEC-14): label, purpose,
 * and active version per key. Rotation is an operator procedure documented in
 * the key-management concept, deliberately not an action here.
 */

const keys = ref<HSMKeyInfo[]>([])
const loading = ref(false)
const error = ref('')

const load = async () => {
  loading.value = true
  error.value = ''
  try {
    keys.value = await keyInventoryService.list()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Could not load the key inventory'
  } finally {
    loading.value = false
  }
}

onMounted(load)

function formatTimestamp(value?: string): string {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}
</script>

<template>
  <div data-testid="key-inventory" class="mx-auto flex w-full max-w-7xl min-w-0 flex-col gap-6 p-4 sm:p-6">
    <div>
      <h1 class="text-2xl font-semibold">Key Inventory</h1>
      <p class="mt-1 opacity-70">
        The HSM-held keys this instance signs and encrypts with. Key rotation is performed by an operator following the
        key management procedure; past versions stay in the HSM for verification of historical signatures.
      </p>
    </div>

    <div v-if="error" data-testid="key-inventory-error" class="alert rounded-box alert-error">{{ error }}</div>

    <section class="min-w-0" aria-labelledby="active-key-inventory-heading">
      <div class="mb-3">
        <h2 id="active-key-inventory-heading" class="text-lg font-semibold">Active key versions</h2>
        <p class="mt-1 text-sm opacity-70">Read-only inventory; key lifecycle actions are performed operationally.</p>
      </div>
      <div v-if="loading" class="opacity-70">Loading…</div>
      <div v-else class="max-w-full overflow-x-auto rounded-box border border-base-300">
        <table class="table min-w-160">
          <thead>
            <tr>
              <th>Label</th>
              <th>Purpose</th>
              <th>Active version</th>
              <th>Last rotated</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="key in keys" :key="key.label" data-testid="key-inventory-row">
              <td class="font-mono text-sm" data-testid="key-inventory-label">{{ key.label }}</td>
              <td>{{ key.purpose }}</td>
              <td>
                <span class="badge badge-ghost badge-sm" :data-testid="`key-version-${key.label}`">
                  v{{ key.active_version }}
                </span>
              </td>
              <td>{{ formatTimestamp(key.updated_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
