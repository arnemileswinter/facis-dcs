import authHttp from '@/api/auth-http'
import type { AuthCallbackResponse } from '@/models/responses/auth-callback-response'
import type { LoginResponse } from '@/models/responses/login-response'
import type { LogoutResponse } from '@/models/responses/logout-response'
import type { AuthenticationService as AuthService } from '@/models/services/authentication-service'
import { useAuthStore } from '@/stores/auth-store'
import { useAuthTokenStore } from '@/stores/auth-token-store'

export const AuthenticationService: AuthService = {
  async getLoginPath() {
    return await authHttp
      .get<LoginResponse>('/auth/login')
      .then((res) => res.data.auth_url)
      .catch((err) => {
        console.error('Login Error:', err)
        return ''
      })
  },

  async refresh() {
    return authHttp
      .post<AuthCallbackResponse>('/auth/refresh')
      .then((res) => {
        const authTokenStore = useAuthTokenStore()
        const resp = res.data
        authTokenStore.setTokens(resp.token_type, resp.access_token)
        const authStore = useAuthStore()
        authStore.setUser(resp.access_token)
        return res.data
      })
      .catch((err) => {
        if (err && err.status === 401) {
          const authStore = useAuthStore()
          authStore.remove()
          const authTokenStore = useAuthTokenStore()
          authTokenStore.remove()
        }
        return Promise.reject(err)
      })
  },

  logout() {
    // Clear local state first
    const authStore = useAuthStore()
    authStore.remove()
    const authTokenStore = useAuthTokenStore()
    authTokenStore.remove()

    // Call backend logout endpoint to get Keycloak logout URL (mirrors login flow)
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
