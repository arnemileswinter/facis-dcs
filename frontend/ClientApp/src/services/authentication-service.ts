import authHttp from '@/api/auth-http'
import type { AuthCallbackResponse, LoginResponse, LogoutResponse } from '@/models/responses/auth-response'
import type { AuthenticationService } from '@/models/services/authentication-service'
import { useAuthStore } from '@/stores/auth-store'
import { useAuthTokenStore } from '@/stores/auth-token-store'

export const authenticationService: AuthenticationService = {
  async loginPath() {
    return await authHttp
      .get<LoginResponse>('/auth/login')
      .then((res) => res.data.auth_url)
      .catch((err) => {
        console.error('Login Error:', err)
        return ''
      })
  },

  async bootstrapStatus() {
    return await authHttp
      .get<{ claimed: boolean }>('/bootstrap/status')
      .then((res) => ({ claimed: !!res.data.claimed }))
      .catch((err) => {
        console.error('Bootstrap Status Error:', err)
        return { claimed: false }
      })
  },

  async bootstrapAdminOffer(holderDid: string) {
    return await authHttp
      .post<{ offer_uri: string }>('/bootstrap/admin-offer', { holder_did: holderDid })
      .then((res) => ({
        offerUri: res.data.offer_uri || '',
        conflict: false,
      }))
      .catch((err) => {
        if (err?.response?.status === 409) {
          // Admin already exists: bootstrap is not needed.
          return { offerUri: '', conflict: true }
        }
        console.error('Bootstrap Offer Error:', err)
        return { offerUri: '', conflict: false }
      })
  },

  async markBootstrapAdminClaimed(holderDid: string) {
    return await authHttp
      .post('/bootstrap/admin-claimed', { holder_did: holderDid })
      .then(() => true)
      .catch((err) => {
        console.error('Bootstrap Claimed Error:', err)
        return false
      })
  },

  async presentationRequestUri(loginChallenge?: string) {
    const params = loginChallenge ? { login_challenge: loginChallenge } : undefined
    return await authHttp
      .get<{ presentation_uri: string }>('/auth/presentation-request', { params })
      .then((res) => res.data.presentation_uri)
      .catch((err) => {
        console.error('Presentation Request Error:', err)
        return ''
      })
  },

  async presentationStatus(requestId: string) {
    return await authHttp
      .get<{ completed: boolean; location?: string }>(`/auth/presentation-status/${encodeURIComponent(requestId)}`)
      .then((res) => ({
        completed: !!res.data.completed,
        location: res.data.location,
      }))
      .catch((err) => {
        console.error('Presentation Status Error:', err)
        return { completed: false }
      })
  },

  async refresh() {
    return authHttp
      .post<AuthCallbackResponse>('/auth/refresh')
      .then((res) => {
        const authTokenStore = useAuthTokenStore()
        authTokenStore.setTokens(res.data.token_type, res.data.access_token)
        const authStore = useAuthStore()
        const userId = authTokenStore.getUserId
        if (!userId) throw new Error('JWT Error')
        authStore.setUser(userId)
        return true
      })
      .catch((err) => {
        if (err && err.status === 401) {
          const authStore = useAuthStore()
          authStore.remove()
          const authTokenStore = useAuthTokenStore()
          authTokenStore.remove()
        }
        return false
      })
  },

  logout() {
    // Clear local state first
    const authStore = useAuthStore()
    authStore.remove()
    const authTokenStore = useAuthTokenStore()
    authTokenStore.remove()

    // Call backend logout endpoint to get the provider logout URL.
    authHttp
      .get<LogoutResponse>('/auth/logout')
      .then((res) => {
        window.location.href = res.data.logout_url
      })
      .catch((err) => {
        console.error('Logout Error:', err)
        // Fallback to home if logout fails
        window.location.href = '/'
      })
  },
}
