import type { AuthCallbackResponse } from "../responses/auth-callback-response"

export interface AuthenticationService {
  getLoginPath: () => Promise<string>
  refresh: () => Promise<AuthCallbackResponse>
  logout: () => void
}
