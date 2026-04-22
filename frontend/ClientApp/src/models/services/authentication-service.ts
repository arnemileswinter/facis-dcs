export interface AuthenticationService {
  loginPath: () => Promise<string>
  bootstrapStatus: () => Promise<{ claimed: boolean }>
  bootstrapAdminOffer: (holderDid: string) => Promise<{ offerUri: string; conflict: boolean }>
  markBootstrapAdminClaimed: (holderDid: string) => Promise<boolean>
  presentationRequestUri: (loginChallenge?: string) => Promise<string>
  presentationStatus: (requestId: string) => Promise<{ completed: boolean; location?: string }>
  refresh: () => Promise<boolean>
  logout: () => void
}
