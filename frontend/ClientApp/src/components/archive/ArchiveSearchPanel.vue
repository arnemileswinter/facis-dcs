<script setup lang="ts">
import { ref } from 'vue'
import { type ArchivedContract, type ArchiveSearchQuery, archiveService } from '@/services/archive-service'
import {
  activeFilters,
  archiveResultsToCsv,
  deleteSavedQuery,
  loadSavedQueries,
  type SavedArchiveQuery,
  saveQuery,
} from '@/utils/archive-search'
import { downloadBlob } from '@/utils/download-blob'

/**
 * Archive search interface (DCS-FR-CSA-22): advanced filters over the
 * indexed archive metadata and content, saved queries, and export of the
 * results for compliance or legal use. Every search is recorded in the
 * archive audit log by the backend (DCS-FR-CSA-18).
 */

const states = ['SIGNED', 'TERMINATED', 'EXPIRED', 'REVOKED']

const query = ref<ArchiveSearchQuery>({})
const validFromDate = ref('')
const validUntilDate = ref('')
const results = ref<ArchivedContract[] | null>(null)
const searching = ref(false)
const error = ref('')
const savedQueries = ref<SavedArchiveQuery[]>(loadSavedQueries())
const queryName = ref('')

function currentQuery(): ArchiveSearchQuery {
  return activeFilters({
    ...query.value,
    valid_from: validFromDate.value ? new Date(`${validFromDate.value}T00:00:00Z`).toISOString() : undefined,
    valid_until: validUntilDate.value ? new Date(`${validUntilDate.value}T23:59:59Z`).toISOString() : undefined,
  })
}

async function search() {
  searching.value = true
  error.value = ''
  try {
    results.value = await archiveService.search(currentQuery())
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    searching.value = false
  }
}

function applySaved(saved: SavedArchiveQuery) {
  const { valid_from, valid_until, ...rest } = saved.query
  query.value = { ...rest }
  validFromDate.value = valid_from?.slice(0, 10) ?? ''
  validUntilDate.value = valid_until?.slice(0, 10) ?? ''
  void search()
}

function save() {
  const name = queryName.value.trim()
  if (!name) return
  savedQueries.value = saveQuery(name, currentQuery())
  queryName.value = ''
}

function remove(name: string) {
  savedQueries.value = deleteSavedQuery(name)
}

function exportResults(format: 'csv' | 'json') {
  if (!results.value) return
  const stamp = new Date().toISOString().slice(0, 10)
  const blob =
    format === 'csv'
      ? new Blob([archiveResultsToCsv(results.value)], { type: 'text/csv' })
      : new Blob([JSON.stringify({ query: currentQuery(), results: results.value }, null, 2)], {
          type: 'application/json',
        })
  downloadBlob(blob, `archive-search-${stamp}.${format}`)
}
</script>

<template>
  <section class="card bg-base-100 shadow" data-testid="archive-search">
    <div class="card-body space-y-3">
      <h2 class="card-title">Search the archive</h2>
      <form class="grid gap-2 md:grid-cols-3" @submit.prevent="search">
        <label class="form-control">
          <span class="label-text">Contract DID</span>
          <input v-model="query.did" class="input-bordered input input-sm" data-testid="archive-search-did" />
        </label>
        <label class="form-control">
          <span class="label-text">State</span>
          <select v-model="query.state" class="select-bordered select select-sm" data-testid="archive-search-state">
            <option value="">Any</option>
            <option v-for="state in states" :key="state" :value="state">{{ state }}</option>
          </select>
        </label>
        <label class="form-control">
          <span class="label-text">Name</span>
          <input v-model="query.name" class="input-bordered input input-sm" />
        </label>
        <label class="form-control">
          <span class="label-text">Party DID</span>
          <input v-model="query.party" class="input-bordered input input-sm" />
        </label>
        <label class="form-control">
          <span class="label-text">Tag</span>
          <input v-model="query.tag" class="input-bordered input input-sm" />
        </label>
        <label class="form-control">
          <span class="label-text">Full text</span>
          <input v-model="query.contract_data" class="input-bordered input input-sm" />
        </label>
        <label class="form-control">
          <span class="label-text">Valid from</span>
          <input v-model="validFromDate" type="date" class="input-bordered input input-sm" />
        </label>
        <label class="form-control">
          <span class="label-text">Valid until</span>
          <input v-model="validUntilDate" type="date" class="input-bordered input input-sm" />
        </label>
        <div class="flex items-end">
          <button type="submit" class="btn btn-sm btn-primary" :disabled="searching" data-testid="archive-search-submit">
            Search
          </button>
        </div>
      </form>

      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="queryName"
          class="input-bordered input input-sm"
          placeholder="Name for this query"
          aria-label="Name for this query"
          data-testid="archive-search-query-name"
        />
        <button class="btn btn-ghost btn-sm" :disabled="!queryName.trim()" data-testid="archive-search-save" @click="save">
          Save query
        </button>
        <span v-for="saved in savedQueries" :key="saved.name" class="badge gap-1 badge-outline">
          <button class="link" :data-testid="`archive-saved-query-${saved.name}`" @click="applySaved(saved)">
            {{ saved.name }}
          </button>
          <button class="link" :aria-label="`Delete saved query ${saved.name}`" @click="remove(saved.name)">×</button>
        </span>
      </div>

      <div v-if="error" class="alert alert-error">{{ error }}</div>
      <template v-else-if="results">
        <div class="flex items-center gap-2">
          <span class="text-sm opacity-70" data-testid="archive-search-count">{{ results.length }} result(s)</span>
          <button class="btn btn-ghost btn-xs" :disabled="results.length === 0" @click="exportResults('csv')">
            Export CSV
          </button>
          <button class="btn btn-ghost btn-xs" :disabled="results.length === 0" @click="exportResults('json')">
            Export JSON
          </button>
        </div>
        <table v-if="results.length > 0" class="table table-zebra table-sm" data-testid="archive-search-results">
          <thead>
            <tr>
              <th>Contract</th>
              <th>Version</th>
              <th>State</th>
              <th>Tags</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="result in results" :key="`${result.did}-${result.contract_version}`">
              <td>
                {{ result.name || result.did }}
                <div class="font-mono text-xs opacity-70">{{ result.did }}</div>
              </td>
              <td>{{ result.contract_version }}</td>
              <td>{{ result.state }}</td>
              <td>{{ result.archive_tags?.join(', ') }}</td>
            </tr>
          </tbody>
        </table>
      </template>
    </div>
  </section>
</template>
