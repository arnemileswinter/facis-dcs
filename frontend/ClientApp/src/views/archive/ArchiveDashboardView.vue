<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ROUTES } from '@/router/router'
import { type ArchiveStatistics, archiveStatisticsService } from '@/services/archive-statistics-service'

/**
 * Contract Archive Dashboard (DCS-FR-CSA-21): an overview of archived
 * contract statistics, recent actions, storage volume, expiring contracts,
 * and compliance status. Rows drill down into the per-contract archive
 * audit trail.
 */

const statistics = ref<ArchiveStatistics | null>(null)
const error = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    statistics.value = await archiveStatisticsService.statistics()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
})

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
  </div>
</template>

<style scoped>
.stat-title,
.stat-desc {
  color: color-mix(in oklab, var(--color-base-content) /* var(--color-base-content) */ 70%, transparent);
}
</style>
