<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import { RouterLink } from 'vue-router'
import ConfirmationModal from '@/components/ConfirmationModal.vue'
import { ROUTES } from '@/router/router'
import {
  type ArchivedContract,
  type ArchiveErasureStatus,
  type ArchiveSearch,
  archiveService,
} from '@/services/archive-service'
import { type ArchiveStatistics, archiveStatisticsService } from '@/services/archive-statistics-service'
import { useAuthStore } from '@/stores/auth-store'
import { useErrorStore } from '@/stores/error-store'
import { contractStates } from '@/types/contract-state'

/**
 * Contract Archive Dashboard (DCS-FR-CSA-21): an overview of archived
 * contract statistics, recent actions, storage volume, expiring contracts,
 * and compliance status. Rows drill down into the per-contract archive
 * audit trail. Archive Managers additionally see the archived-contract
 * entries with their key-erasure state and the delete-with-shred action
 * (DCS-FR-CSA-17, DCS-NFR-SEC-13).
 */

const statistics = ref<ArchiveStatistics | null>(null)
const error = ref('')
const loading = ref(true)

const authStore = useAuthStore()
const errorStore = useErrorStore()
const isArchiveManager = computed(() => authStore.user?.roles.includes('ARCHIVE_MANAGER') ?? false)

const canReadEntries = computed(() => isArchiveManager.value || !!authStore.user?.roles.includes('CONTRACT_OBSERVER'))
const canReadStatistics = computed(() => isArchiveManager.value || !!authStore.user?.roles.includes('AUDITOR'))
const search = ref<ArchiveSearch>({})
const entriesLoading = ref(false)
let entriesRequest = 0
onBeforeUnmount(() => entriesRequest++)
const entries = ref<ArchivedContract[]>([])
const entriesError = ref('')
const erasureStatuses = ref<Record<string, ArchiveErasureStatus>>({})
const expandedErasureDid = ref<string | null>(null)
const deleting = ref(false)
const confirmationModal = useTemplateRef('confirmation-modal')

onMounted(async () => {
  try {
    if (canReadStatistics.value) statistics.value = await archiveStatisticsService.statistics()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
  if (canReadEntries.value) {
    await loadEntries()
  }
})

function resetSearch() {
  search.value = {}
  void loadEntries()
}

async function loadEntries() {
  const request = ++entriesRequest
  entriesLoading.value = true
  entriesError.value = ''
  entries.value = []
  erasureStatuses.value = {}
  expandedErasureDid.value = null
  try {
    const filters = Object.fromEntries(
      Object.entries(search.value).filter(([, value]) => value !== '' && value != null),
    )
    for (const key of ['valid_from', 'valid_until']) {
      if (typeof filters[key] === 'string') filters[key] = new Date(String(filters[key])).toISOString()
    }
    const result = Object.keys(filters).length ? await archiveService.search(filters) : await archiveService.retrieve()
    if (request !== entriesRequest) return
    entries.value = result
    if (isArchiveManager.value) await loadErasureStatuses(request)
  } catch (err) {
    if (request === entriesRequest) entriesError.value = err instanceof Error ? err.message : String(err)
  } finally {
    if (request === entriesRequest) entriesLoading.value = false
  }
}

async function loadErasureStatuses(request: number) {
  const dids = Array.from(new Set(entries.value.map((entry) => entry.did)))
  const results = await Promise.allSettled(dids.map((did) => archiveService.erasureStatus(did)))
  const statuses: Record<string, ArchiveErasureStatus> = {}
  for (const result of results) {
    if (result.status === 'fulfilled') statuses[result.value.did] = result.value
  }
  if (request === entriesRequest) erasureStatuses.value = statuses
}

function toggleErasureDetails(did: string) {
  expandedErasureDid.value = expandedErasureDid.value === did ? null : did
}

const expandedStatus = computed(() =>
  expandedErasureDid.value ? (erasureStatuses.value[expandedErasureDid.value] ?? null) : null,
)

const annotating = ref(false)

// DCS-FR-CSA-11: summary and tags are the only mutable part of an archive
// entry. The dialog takes the summary; tags are entered on the same line,
// comma-separated, because the shared modal offers a single text field.
async function annotateEntry(entry: ArchivedContract) {
  const dialog = confirmationModal.value
  if (!dialog) return
  const { isCanceled, data: summary } = await dialog.reveal({
    message: `Summary for "${entry.name?.trim() ?? entry.did}". Leave empty to have one generated from the contract metadata.`,
    editor: { requiredText: false, placeholder: 'Summary' },
  })
  if (isCanceled) return
  const { isCanceled: tagsCanceled, data: tags } = await dialog.reveal({
    message: 'Tags for this entry, comma-separated. Leave empty to keep the current set.',
    editor: { requiredText: false, placeholder: 'e.g. supply, 2026, renewed' },
  })
  if (tagsCanceled) return
  const tagList = (tags ?? '')
    .split(',')
    .map((tag) => tag.trim())
    .filter((tag) => tag.length > 0)
  annotating.value = true
  try {
    const trimmedSummary = summary?.trim()
    await archiveService.annotate(
      entry.did,
      trimmedSummary === '' ? undefined : trimmedSummary,
      tagList.length > 0 ? tagList : undefined,
    )
    await loadEntries()
  } catch {
    // the shared http interceptor already surfaced the server's error
  } finally {
    annotating.value = false
  }
}

async function deleteEntry(entry: ArchivedContract) {
  const dialog = confirmationModal.value
  if (!dialog) return
  const displayName = entry.name?.trim() ? entry.name : entry.did
  const { isCanceled, data: justification } = await dialog.reveal({
    message: `Delete the archive entry for "${displayName}"? A justification is required and is recorded with the deletion.`,
    editor: { requiredText: true, placeholder: 'Justification' },
    acknowledgement: 'Encryption keys will be destroyed on both instances (irreversible)',
  })
  if (isCanceled || !justification) return
  deleting.value = true
  try {
    await archiveService.delete(entry.did, justification)
    errorStore.add('Archive entry deleted. Encryption keys destroyed.', 'info')
    if (canReadStatistics.value) statistics.value = await archiveStatisticsService.statistics()
    await loadEntries()
  } catch {
    // the shared http interceptor already surfaced the server's error
  } finally {
    deleting.value = false
  }
}

const storageDisplay = computed(() => {
  const bytes = statistics.value?.storage_bytes ?? 0
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${bytes} B`
})

function formatTimestamp(value: string): string {
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

function shortDid(did: string): string {
  return did.length > 24 ? `${did.slice(0, 12)}…${did.slice(-8)}` : did
}
</script>

<template>
  <div class="space-y-6 p-4" data-testid="archive-dashboard">
    <h1 class="text-2xl font-bold">Contract Archive Dashboard</h1>

    <div v-if="loading" class="flex justify-center p-8">
      <span class="loading loading-lg loading-spinner" />
    </div>
    <div v-else-if="error" class="alert alert-error" data-testid="archive-dashboard-error">{{ error }}</div>

    <template v-else-if="statistics">
      <!-- Archived contract statistics + storage volume + compliance status -->
      <div class="stats w-full stats-vertical shadow lg:stats-horizontal" data-testid="archive-statistics">
        <div class="stat">
          <div class="stat-title">Archived contracts</div>
          <div class="stat-value" data-testid="stat-archived-contracts">{{ statistics.archived_contracts }}</div>
          <div class="stat-desc">{{ statistics.archived_total }} entries across all versions</div>
        </div>
        <div class="stat">
          <div class="stat-title">Storage volume</div>
          <div class="stat-value text-primary" data-testid="stat-storage-volume">{{ storageDisplay }}</div>
          <div class="stat-desc">snapshots and evidence</div>
        </div>
        <div class="stat">
          <div class="stat-title">Compliant</div>
          <div class="stat-value text-success" data-testid="stat-compliant">{{ statistics.compliant_total }}</div>
          <div class="stat-desc">complete evidence sets</div>
        </div>
        <div class="stat">
          <div class="stat-title">Flagged</div>
          <div class="stat-value" :class="statistics.flagged_total > 0 ? 'text-error' : ''" data-testid="stat-flagged">
            {{ statistics.flagged_total }}
          </div>
          <div class="stat-desc">incomplete evidence (DCS-FR-CSA-19)</div>
        </div>
        <div class="stat">
          <div class="stat-title">Deleted</div>
          <div class="stat-value" data-testid="stat-deleted">{{ statistics.deleted_total }}</div>
          <div class="stat-desc">soft-deleted entries</div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-2">
        <!-- Expiring contracts -->
        <section class="card bg-base-100 shadow" data-testid="archive-expiring">
          <div class="card-body">
            <h2 class="card-title">Expiring within 30 days</h2>
            <p v-if="statistics.expiring_contracts.length === 0" class="text-sm opacity-70">
              No archived contracts expire within the next 30 days.
            </p>
            <table v-else class="table table-zebra table-sm">
              <thead>
                <tr>
                  <th>Contract</th>
                  <th>Expires</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="contract in statistics.expiring_contracts" :key="contract.did">
                  <td>
                    <RouterLink
                      :to="{ name: ROUTES.AUDIT.LIST, query: { scope: 'archive', did: contract.did } }"
                      class="link link-primary"
                      :data-testid="`expiring-${contract.did}`"
                    >
                      {{ contract.name || shortDid(contract.did) }}
                    </RouterLink>
                  </td>
                  <td>{{ formatTimestamp(contract.exp_date) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <!-- Recent actions -->
        <section class="card bg-base-100 shadow" data-testid="archive-recent-actions">
          <div class="card-body">
            <h2 class="card-title">Recent actions</h2>
            <p v-if="statistics.recent_actions.length === 0" class="text-sm opacity-70">
              No archive operations recorded yet.
            </p>
            <table v-else class="table table-zebra table-sm">
              <thead>
                <tr>
                  <th>When</th>
                  <th>Operation</th>
                  <th>Actor</th>
                  <th>Contract</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(action, index) in statistics.recent_actions" :key="index">
                  <td>{{ formatTimestamp(action.occurred_at) }}</td>
                  <td>{{ action.event_type }}</td>
                  <td class="max-w-40 truncate" :title="action.actor">{{ action.actor }}</td>
                  <td>
                    <RouterLink
                      v-if="action.did"
                      :to="{ name: ROUTES.AUDIT.LIST, query: { scope: 'archive', did: action.did } }"
                      class="link link-primary"
                    >
                      {{ shortDid(action.did) }}
                    </RouterLink>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </template>

    <!-- Archived contract entries with key-erasure state and delete-with-shred -->
    <section v-if="canReadEntries" class="card bg-base-100 shadow" data-testid="archive-entries">
      <div class="card-body">
        <h2 class="card-title">Archived contracts</h2>
        <form class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4" @submit.prevent="loadEntries">
          <label class="fieldset">
            Name
            <input v-model="search.name" class="input w-full" />
          </label>
          <label class="fieldset">
            DID
            <input v-model="search.did" class="input w-full" />
          </label>
          <label class="fieldset">
            Party
            <input v-model="search.party" class="input w-full" />
          </label>
          <label class="fieldset">
            State
            <select v-model="search.state" class="select w-full">
              <option :value="undefined">All states</option>
              <option v-for="state in contractStates" :key="state" :value="state">{{ state }}</option>
            </select>
          </label>
          <label class="fieldset">
            Tag
            <input v-model="search.tag" class="input w-full" />
          </label>
          <label class="fieldset">
            Version
            <input v-model.number="search.contract_version" type="number" min="1" class="input w-full" />
          </label>
          <label class="fieldset">
            Valid from
            <input v-model="search.valid_from" type="date" class="input w-full" />
          </label>
          <label class="fieldset">
            Valid until
            <input v-model="search.valid_until" type="date" class="input w-full" />
          </label>
          <button class="btn btn-primary" type="submit" :disabled="entriesLoading">Search archive</button>
          <button class="btn btn-outline" type="button" @click="resetSearch">Reset search</button>
        </form>
        <p v-if="entriesLoading" role="status">Loading archived contracts…</p>
        <div v-else-if="entriesError" class="alert alert-error" data-testid="archive-entries-error">
          {{ entriesError }}
        </div>
        <p v-else-if="entries.length === 0" class="text-sm opacity-70" data-testid="archive-entries-empty">
          No archived contracts.
        </p>
        <div v-else class="overflow-x-auto">
          <table class="table table-zebra table-sm">
            <thead>
              <tr>
                <th>Contract</th>
                <th>Version</th>
                <th>State</th>
                <th>Annotation</th>
                <th v-if="isArchiveManager">Encryption</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <template v-for="entry in entries" :key="`${entry.did}-${entry.contract_version}`">
                <tr :data-testid="`archive-entry-${entry.did}`">
                  <td>
                    <RouterLink
                      :to="{
                        name: ROUTES.ARCHIVE.DETAIL,
                        query: { did: entry.did, contract_version: entry.contract_version },
                      }"
                      class="link link-primary"
                    >
                      {{ entry.name || shortDid(entry.did) }}
                    </RouterLink>
                    <div class="font-mono text-xs opacity-70">{{ shortDid(entry.did) }}</div>
                  </td>
                  <td>{{ entry.contract_version }}</td>
                  <td>
                    <span class="badge badge-ghost badge-sm">{{ entry.state }}</span>
                  </td>
                  <td :data-testid="`archive-annotation-${entry.did}`">
                    <div v-if="entry.archive_summary" class="max-w-xs truncate text-sm">
                      {{ entry.archive_summary }}
                    </div>
                    <div v-else class="text-sm opacity-50">Not annotated</div>
                    <div v-if="entry.archive_tags?.length" class="mt-1 flex flex-wrap gap-1">
                      <span v-for="tag in entry.archive_tags" :key="tag" class="badge badge-ghost badge-xs">
                        {{ tag }}
                      </span>
                    </div>
                  </td>
                  <td v-if="isArchiveManager">
                    <button
                      v-if="erasureStatuses[entry.did]?.local_status === 'shredded'"
                      type="button"
                      class="badge cursor-pointer badge-sm badge-error"
                      :data-testid="`erasure-badge-${entry.did}`"
                      @click="toggleErasureDetails(entry.did)"
                    >
                      Keys destroyed
                    </button>
                    <button
                      v-else-if="erasureStatuses[entry.did]"
                      type="button"
                      class="badge cursor-pointer badge-ghost badge-sm"
                      :data-testid="`erasure-badge-${entry.did}`"
                      @click="toggleErasureDetails(entry.did)"
                    >
                      Keys live
                    </button>
                  </td>
                  <td v-if="isArchiveManager" class="text-right">
                    <RouterLink
                      :to="{ name: ROUTES.AUDIT.LIST, query: { scope: 'archive', did: entry.did } }"
                      class="link"
                    >
                      Audit
                    </RouterLink>
                    <button
                      class="btn btn-ghost btn-xs"
                      :disabled="annotating"
                      :data-testid="`archive-annotate-${entry.did}`"
                      @click="annotateEntry(entry)"
                    >
                      Annotate
                    </button>
                    <button
                      class="btn text-error btn-ghost btn-xs"
                      :disabled="deleting"
                      :data-testid="`archive-delete-${entry.did}`"
                      @click="deleteEntry(entry)"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
                <tr v-if="expandedErasureDid === entry.did && expandedStatus">
                  <td colspan="6">
                    <div class="space-y-2 p-2 text-sm" :data-testid="`erasure-details-${entry.did}`">
                      <template v-if="expandedStatus.local_status === 'shredded'">
                        <div>
                          <span class="font-medium">Keys destroyed</span>
                          {{ formatTimestamp(expandedStatus.shredded_at ?? '') }}
                          by {{ expandedStatus.shredded_by }}:
                          {{ expandedStatus.shred_reason }}
                        </div>
                      </template>
                      <div v-else>Encryption keys are live. The archived content is decryptable.</div>
                      <table v-if="expandedStatus.peers.length > 0" class="table table-xs">
                        <thead>
                          <tr>
                            <th>Peer</th>
                            <th>Status</th>
                            <th>Requested</th>
                            <th>Confirmed</th>
                            <th>Retries</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="peer in expandedStatus.peers" :key="peer.peer_did">
                            <td class="font-mono text-xs">{{ peer.peer_did }}</td>
                            <td>
                              <span
                                class="badge badge-sm"
                                :class="peer.status === 'confirmed' ? 'badge-success' : 'badge-warning'"
                              >
                                {{ peer.status }}
                              </span>
                            </td>
                            <td>{{ formatTimestamp(peer.requested_at) }}</td>
                            <td>{{ peer.confirmed_at ? formatTimestamp(peer.confirmed_at) : '—' }}</td>
                            <td>
                              {{ peer.retry_count }}
                              <span v-if="peer.last_tried_at" class="opacity-70">
                                (last tried {{ formatTimestamp(peer.last_tried_at) }})
                              </span>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <ConfirmationModal ref="confirmation-modal" />
  </div>
</template>

<style scoped>
.stat-title,
.stat-desc {
  color: color-mix(in oklab, var(--color-base-content) /* var(--color-base-content) */ 70%, transparent);
}
</style>
