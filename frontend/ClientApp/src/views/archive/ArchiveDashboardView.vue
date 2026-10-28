<script setup lang="ts">
import { ChevronDownIcon, MagnifyingGlassIcon } from '@heroicons/vue/24/outline'
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
import { claimReportedHttpError } from '@/utils/report-action-error'

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
const showAdditionalFilters = ref(false)
const additionalFilterCount = computed(
  () =>
    Object.entries(search.value).filter(
      ([key, value]) => !['name', 'state'].includes(key) && value !== '' && value != null,
    ).length,
)
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
const annotationDialog = useTemplateRef('annotation-dialog')
const annotationEntry = ref<ArchivedContract | null>(null)
const annotationSummary = ref('')
const annotationTags = ref('')
const annotationError = ref('')

// DCS-FR-CSA-11: summary and tags are the only mutable part of an archive entry.
function annotateEntry(entry: ArchivedContract) {
  annotationEntry.value = entry
  annotationSummary.value = entry.archive_summary ?? ''
  annotationTags.value = entry.archive_tags?.join(', ') ?? ''
  annotationError.value = ''
  annotationDialog.value?.showModal()
}

async function saveAnnotation() {
  const entry = annotationEntry.value
  if (!entry || annotating.value) return
  const summary = annotationSummary.value.trim()
  const tags = annotationTags.value
    .split(',')
    .map((tag) => tag.trim())
    .filter((tag) => tag.length > 0)
  annotating.value = true
  annotationError.value = ''
  try {
    await archiveService.annotate(entry.did, summary || undefined, tags.length > 0 ? tags : undefined)
  } catch (err) {
    // The dialog stays open to keep the input. If the user closed it during the request, the toast stays.
    if (annotationDialog.value?.open) {
      claimReportedHttpError(err)
      annotationError.value = err instanceof Error ? err.message : String(err)
    }
    return
  } finally {
    annotating.value = false
  }
  annotationDialog.value?.close()
  errorStore.add('Annotation saved.', 'info')
  await loadEntries()
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
  <div class="mx-auto w-full max-w-screen-2xl min-w-0 space-y-6 p-4 lg:p-6" data-testid="archive-dashboard">
    <header>
      <h1 class="text-2xl font-bold">Contract Archive Dashboard</h1>
      <p class="mt-1 text-sm text-base-content/70">
        Find archived contracts, review evidence and track archive activity.
      </p>
    </header>

    <!-- Archived contract entries with key-erasure state and delete-with-shred -->
    <section
      v-if="canReadEntries"
      class="card min-w-0 border border-base-300 bg-base-100 shadow-sm"
      data-testid="archive-entries"
    >
      <div class="card-body min-w-0 p-4 sm:p-6">
        <h2 class="card-title">Archived contracts</h2>
        <p class="text-sm text-base-content/70">Search the archive and open a preserved contract version.</p>
        <form class="my-3 space-y-3" @submit.prevent="loadEntries">
          <div class="grid items-end gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,0.5fr)_auto]">
            <label class="fieldset min-w-0">
              Name
              <input v-model="search.name" class="input w-full" placeholder="Search by contract name" />
            </label>
            <label class="fieldset min-w-0">
              State
              <select v-model="search.state" class="select w-full">
                <option :value="undefined">All states</option>
                <option v-for="state in contractStates" :key="state" :value="state">{{ state }}</option>
              </select>
            </label>
            <button class="btn mb-1 btn-primary" type="submit" :disabled="entriesLoading">
              <MagnifyingGlassIcon class="size-4" aria-hidden="true" />
              Search archive
            </button>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="btn btn-ghost btn-sm"
              :aria-expanded="showAdditionalFilters"
              aria-controls="archive-additional-filters"
              @click="showAdditionalFilters = !showAdditionalFilters"
            >
              <ChevronDownIcon
                class="size-4 transition-transform"
                :class="{ 'rotate-180': showAdditionalFilters }"
                aria-hidden="true"
              />
              Additional filters
              <span v-if="additionalFilterCount" class="badge badge-sm badge-primary">{{ additionalFilterCount }}</span>
            </button>
            <button class="btn btn-ghost btn-sm" type="button" @click="resetSearch">Reset search</button>
          </div>
          <div
            v-show="showAdditionalFilters"
            id="archive-additional-filters"
            class="grid gap-3 rounded-box bg-base-200/50 p-4 sm:grid-cols-2 lg:grid-cols-3"
          >
            <label class="fieldset min-w-0">
              DID
              <input v-model="search.did" class="input w-full" />
            </label>
            <label class="fieldset min-w-0">
              Party
              <input v-model="search.party" class="input w-full" />
            </label>
            <label class="fieldset min-w-0">
              Tag
              <input v-model="search.tag" class="input w-full" />
            </label>
            <label class="fieldset min-w-0">
              Version
              <input v-model.number="search.contract_version" type="number" min="1" class="input w-full" />
            </label>
            <label class="fieldset min-w-0">
              Valid from
              <input v-model="search.valid_from" type="date" class="input w-full" />
            </label>
            <label class="fieldset min-w-0">
              Valid until
              <input v-model="search.valid_until" type="date" class="input w-full" />
            </label>
          </div>
        </form>
        <p v-if="entriesLoading" role="status">Loading archived contracts…</p>
        <div v-else-if="entriesError" class="alert alert-error" data-testid="archive-entries-error">
          {{ entriesError }}
        </div>
        <p v-else-if="entries.length === 0" class="text-sm opacity-70" data-testid="archive-entries-empty">
          No archived contracts.
        </p>
        <div v-else class="min-w-0">
          <table class="archive-contracts table w-full table-fixed">
            <thead>
              <tr>
                <th class="w-1/3">Contract</th>
                <th class="w-20">Version</th>
                <th>State</th>
                <th>Annotation</th>
                <th v-if="isArchiveManager">Encryption</th>
                <th v-if="isArchiveManager"><span class="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              <template v-for="entry in entries" :key="`${entry.did}-${entry.contract_version}`">
                <tr class="archive-entry" :data-testid="`archive-entry-${entry.did}`">
                  <td>
                    <RouterLink
                      :to="{
                        name: ROUTES.ARCHIVE.DETAIL,
                        query: { did: entry.did, contract_version: entry.contract_version },
                      }"
                      class="archive-entry-link font-semibold wrap-anywhere"
                      :aria-label="`${entry.name || entry.did}, version ${entry.contract_version}`"
                    >
                      {{ entry.name || shortDid(entry.did) }}
                    </RouterLink>
                    <div class="mt-1 font-mono text-xs opacity-70" :title="entry.did">{{ shortDid(entry.did) }}</div>
                  </td>
                  <td>
                    <span class="mr-1 text-base-content/70 lg:hidden">Version</span>
                    {{ entry.contract_version }}
                  </td>
                  <td>
                    <span class="badge badge-ghost badge-sm">{{ entry.state }}</span>
                  </td>
                  <td :data-testid="`archive-annotation-${entry.did}`">
                    <div
                      v-if="entry.archive_summary"
                      class="line-clamp-2 text-sm wrap-anywhere"
                      :title="entry.archive_summary"
                    >
                      {{ entry.archive_summary }}
                    </div>
                    <div v-else class="text-sm opacity-50">Not annotated</div>
                    <div v-if="entry.archive_tags?.length" class="mt-1 flex flex-wrap gap-1">
                      <span
                        v-for="tag in entry.archive_tags"
                        :key="tag"
                        class="badge block max-w-full truncate badge-ghost badge-xs"
                        :title="tag"
                      >
                        {{ tag }}
                      </span>
                    </div>
                  </td>
                  <td v-if="isArchiveManager">
                    <button
                      v-if="erasureStatuses[entry.did]?.local_status === 'shredded'"
                      type="button"
                      class="btn h-auto min-h-8 btn-outline btn-sm btn-error"
                      :aria-expanded="expandedErasureDid === entry.did"
                      :data-testid="`erasure-badge-${entry.did}`"
                      @click="toggleErasureDetails(entry.did)"
                    >
                      Keys destroyed
                    </button>
                    <button
                      v-else-if="erasureStatuses[entry.did]"
                      type="button"
                      class="btn h-auto min-h-8 btn-ghost btn-sm"
                      :aria-expanded="expandedErasureDid === entry.did"
                      :data-testid="`erasure-badge-${entry.did}`"
                      @click="toggleErasureDetails(entry.did)"
                    >
                      Keys live
                    </button>
                  </td>
                  <td v-if="isArchiveManager" class="space-y-1">
                    <RouterLink
                      :to="{ name: ROUTES.AUDIT.LIST, query: { scope: 'archive', did: entry.did } }"
                      class="btn btn-ghost btn-sm"
                    >
                      Audit
                    </RouterLink>
                    <button
                      class="btn btn-ghost btn-sm"
                      :disabled="annotating"
                      :data-testid="`archive-annotate-${entry.did}`"
                      @click="annotateEntry(entry)"
                    >
                      Annotate
                    </button>
                    <button
                      class="btn text-error btn-ghost btn-sm"
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
                    <div
                      class="space-y-2 overflow-x-auto p-2 text-sm wrap-anywhere"
                      :data-testid="`erasure-details-${entry.did}`"
                    >
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

    <h2 v-if="canReadStatistics" class="text-lg font-semibold">Archive overview</h2>

    <div v-if="loading" class="flex justify-center p-8">
      <span class="loading loading-lg loading-spinner" />
    </div>
    <div v-else-if="error" class="alert alert-error" data-testid="archive-dashboard-error">{{ error }}</div>

    <template v-else-if="statistics">
      <!-- Archived contract statistics + storage volume + compliance status -->
      <div class="grid min-w-0 grid-cols-2 gap-3 lg:grid-cols-5" data-testid="archive-statistics">
        <div class="stat min-w-0 rounded-box border border-base-300 bg-base-100 p-4">
          <div class="stat-title">Archived contracts</div>
          <div class="stat-value text-2xl" data-testid="stat-archived-contracts">
            {{ statistics.archived_contracts }}
          </div>
          <div class="stat-desc whitespace-normal">{{ statistics.archived_total }} entries across all versions</div>
        </div>
        <div class="stat min-w-0 rounded-box border border-base-300 bg-base-100 p-4">
          <div class="stat-title">Storage volume</div>
          <div class="stat-value text-2xl text-primary" data-testid="stat-storage-volume">{{ storageDisplay }}</div>
          <div class="stat-desc whitespace-normal">snapshots and evidence</div>
        </div>
        <div class="stat min-w-0 rounded-box border border-base-300 bg-base-100 p-4">
          <div class="stat-title">Compliant</div>
          <div class="stat-value text-2xl text-success" data-testid="stat-compliant">
            {{ statistics.compliant_total }}
          </div>
          <div class="stat-desc whitespace-normal">complete evidence sets</div>
        </div>
        <div
          class="stat min-w-0 rounded-box border bg-base-100 p-4"
          :class="statistics.flagged_total > 0 ? 'border-error bg-error/5' : 'border-base-300'"
        >
          <div class="stat-title">Flagged</div>
          <div
            class="stat-value text-2xl"
            :class="statistics.flagged_total > 0 ? 'text-error' : ''"
            data-testid="stat-flagged"
          >
            {{ statistics.flagged_total }}
          </div>
          <div class="stat-desc whitespace-normal">incomplete evidence</div>
        </div>
        <div class="stat min-w-0 rounded-box border border-base-300 bg-base-100 p-4">
          <div class="stat-title">Deleted</div>
          <div class="stat-value text-2xl" data-testid="stat-deleted">{{ statistics.deleted_total }}</div>
          <div class="stat-desc whitespace-normal">soft-deleted entries</div>
        </div>
      </div>

      <div class="grid items-start gap-6 lg:grid-cols-2">
        <!-- Expiring contracts -->
        <section
          class="card min-w-0 border border-base-300 bg-base-100"
          :class="{ 'border-warning': statistics.expiring_contracts.length > 0 }"
          data-testid="archive-expiring"
        >
          <div class="card-body min-w-0 p-4 sm:p-6">
            <h2 class="card-title text-base">
              Expiring within 30 days
              <span v-if="statistics.expiring_contracts.length" class="badge badge-sm badge-warning">
                {{ statistics.expiring_contracts.length }}
              </span>
            </h2>
            <p v-if="statistics.expiring_contracts.length === 0" class="text-sm opacity-70">
              No archived contracts expire within the next 30 days.
            </p>
            <div v-else class="overflow-x-auto" tabindex="0" role="region" aria-label="Expiring contracts">
              <table class="table table-zebra table-sm">
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
          </div>
        </section>

        <!-- Recent actions -->
        <section class="card min-w-0 border border-base-300 bg-base-100" data-testid="archive-recent-actions">
          <div class="card-body min-w-0 p-4 sm:p-6">
            <h2 class="card-title text-base">Recent actions</h2>
            <p v-if="statistics.recent_actions.length === 0" class="text-sm opacity-70">
              No archive operations recorded yet.
            </p>
            <div v-else class="overflow-x-auto" tabindex="0" role="region" aria-label="Recent archive actions">
              <table class="table table-zebra table-sm">
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
          </div>
        </section>
      </div>
    </template>

    <ConfirmationModal ref="confirmation-modal" />
    <dialog
      ref="annotation-dialog"
      class="modal modal-bottom transition-none sm:modal-middle"
      aria-labelledby="archive-annotation-title"
      aria-describedby="archive-annotation-description"
      data-testid="archive-annotation-dialog"
    >
      <form class="modal-box" @submit.prevent="saveAnnotation">
        <h3 id="archive-annotation-title" class="text-lg font-bold">Edit annotation</h3>
        <p id="archive-annotation-description" class="py-4">
          Annotation for "{{ annotationEntry?.name?.trim() || annotationEntry?.did }}". Leave the summary empty to keep
          the existing summary. If none exists, one is generated from the contract metadata. Leave the tags empty to
          keep the current set.
        </p>
        <div class="flex flex-col gap-3">
          <label for="archive-annotation-summary" class="text-sm font-medium">Summary</label>
          <textarea
            id="archive-annotation-summary"
            v-model="annotationSummary"
            class="textarea w-full"
            placeholder="Summary"
            rows="4"
          />
          <label for="archive-annotation-tags" class="text-sm font-medium">Tags</label>
          <input
            id="archive-annotation-tags"
            v-model="annotationTags"
            class="input w-full"
            placeholder="e.g. supply, 2026, renewed"
          />
        </div>
        <div v-if="annotationError" role="alert" class="mt-3 alert alert-error">
          {{ annotationError }}
        </div>
        <div class="modal-action">
          <button type="button" class="btn btn-outline btn-sm" @click="annotationDialog?.close()">Cancel</button>
          <button type="submit" class="btn btn-sm btn-primary" :disabled="annotating">Save annotation</button>
        </div>
      </form>
    </dialog>
  </div>
</template>

<style scoped>
.archive-entry {
  position: relative;
}
.archive-entry:hover {
  background: var(--color-base-200);
}
.archive-entry-link,
.archive-entry-link:hover {
  text-decoration: none;
}
.archive-entry-link::after {
  position: absolute;
  z-index: 1;
  inset: 0;
  content: '';
}
.archive-entry-link:focus-visible {
  outline: none;
}
.archive-entry-link:focus-visible::after {
  outline: 2px solid var(--color-primary);
  outline-offset: -2px;
}
.archive-entry button,
.archive-entry a:not(.archive-entry-link) {
  position: relative;
  z-index: 2;
}

@media (max-width: 1023px) {
  .archive-contracts,
  .archive-contracts tbody,
  .archive-contracts tr,
  .archive-contracts td {
    display: block;
    width: 100%;
  }
  .archive-contracts thead {
    display: none;
  }
  .archive-contracts tr {
    padding: 1rem 0;
  }
  .archive-contracts td {
    padding: 0.375rem 0;
  }
}

.stat-title,
.stat-desc {
  color: color-mix(in oklab, var(--color-base-content) /* var(--color-base-content) */ 70%, transparent);
}
</style>
