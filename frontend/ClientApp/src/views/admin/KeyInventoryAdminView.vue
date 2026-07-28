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
  <div data-testid="key-inventory" class="flex flex-col gap-6 p-6">
    <div>
      <h1 class="text-2xl font-semibold">Key Inventory</h1>
      <p class="mt-1 opacity-70">
        The HSM-held keys this instance signs and encrypts with. Key rotation is performed by an operator following the
        key management procedure; past versions stay in the HSM for verification of historical signatures.
      </p>
    </div>

    <div v-if="error" data-testid="key-inventory-error" class="alert rounded-box alert-error">{{ error }}</div>

    <section>
      <div v-if="loading" class="opacity-70">Loading…</div>
      <div v-else class="overflow-x-auto">
        <table class="table">
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
                <!-- v-text, not interpolation: a text node would carry the
                     template's surrounding indentation into the badge. -->
                <span
                  class="badge badge-ghost badge-sm"
                  :data-testid="`key-version-${key.label}`"
                  v-text="`v${key.active_version}`"
                />
              </td>
              <td>{{ formatTimestamp(key.updated_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
