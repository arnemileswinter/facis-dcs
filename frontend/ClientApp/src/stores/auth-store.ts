import { users } from '@/services/user-service'
import type { UserRole } from '@/types/user-role'
import { useJwt } from '@vueuse/integrations/useJwt'
import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'
import { useAuthTokenStore } from './auth-token-store'

interface User {
  id: string
  username: string
  name: string
  roles?: UserRole[]
}

interface AccessTokenPayload {
  sub?: string
  ext?: { roles?: string[] }
  roles?: string[]
}

// Backend / Hydra emit role names as design Scope() display strings
// (e.g. "Template Creator", "Sys. Contract Manager"). The frontend
// guards on the uppercase enum IDs in `UserRole`. Translate here so
// the store always exposes the canonical IDs.
const ROLE_NAME_TO_ID: Record<string, UserRole> = {
  'Template Creator': 'TEMPLATE_CREATOR',
  'Template Reviewer': 'TEMPLATE_REVIEWER',
  'Template Approver': 'TEMPLATE_APPROVER',
  'Template Manager': 'TEMPLATE_MANAGER',
  'Contract Creator': 'CONTRACT_CREATOR',
  'Contract Negotiator': 'CONTRACT_NEGOTIATOR',
  'Contract Reviewer': 'CONTRACT_REVIEWER',
  'Contract Approver': 'CONTRACT_APPROVER',
  'Contract Manager': 'CONTRACT_MANAGER',
  'Contract Signer': 'CONTRACT_SIGNER',
  'Contract Observer': 'CONTRACT_OBSERVER',
  'Sys. Contract Creator': 'CONTRACT_CREATOR',
  'Sys. Contract Reviewer': 'CONTRACT_REVIEWER',
  'Sys. Contract Approver': 'CONTRACT_APPROVER',
  'Sys. Contract Manager': 'CONTRACT_MANAGER',
  'Sys. Contract Signer': 'CONTRACT_SIGNER',
  'Archive Manager': 'ARCHIVE_MANAGER',
  Auditor: 'AUDITOR',
  'System Administrator': 'SYSTEM_ADMINISTRATOR',
  'Compliance Officer': 'COMPLIANCE_OFFICER',
}

function normalizeRoles(raw: readonly string[] | undefined): UserRole[] {
  if (!raw) return []
  const seen = new Set<UserRole>()
  for (const name of raw) {
    const id = ROLE_NAME_TO_ID[name] ?? (name as UserRole)
    seen.add(id)
  }
  return Array.from(seen)
}

export const useAuthStore = defineStore('auth', () => {
  const authTokenStore = useAuthTokenStore()
  const user: Ref<User | null> = ref(null)

  const isAuthenticated = computed(() => !!user.value && authTokenStore.isAuthSet)

  function setUser(userId: string) {
    const userProfile = users.value.find((user) => user.id === userId)
    debugger
    if (userProfile) {
      user.value = {
        id: userProfile.id,
        username: userProfile.username,
        name: userProfile.firstName + ' ' + userProfile.lastName,
        roles: userProfile.roleIds,
      }
      return
    }

    // No mock profile matches: synthesize a user from the access-token claims.
    // Wallet-driven login uses the holder DID as `sub`, which won't appear in
    // the mock catalogue; roles come from the Hydra session we set during
    // login/consent accept (exposed under `ext.roles` in JWT access tokens).
    const payload = useJwt<AccessTokenPayload>(authTokenStore.accessToken).payload.value
    const roles = normalizeRoles(payload?.ext?.roles ?? payload?.roles)
    user.value = {
      id: userId,
      username: userId,
      name: userId,
      roles,
    }
  }

  function remove() {
    user.value = null
  }

  return { user, isAuthenticated, setUser, remove }
})
