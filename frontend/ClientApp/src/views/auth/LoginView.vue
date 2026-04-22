<script setup lang="ts">
import { ROUTES } from '@/router/router'
import { authenticationService } from '@/services/authentication-service'
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import QRCode from 'qrcode'

const route = useRoute()
const router = useRouter()
const presentationUri = ref<string>('')
const offerUri = ref<string>('')
const holderDid = ref<string>('')
const bootstrapError = ref<string>('')
const step = ref<'bootstrap' | 'offer' | 'presentation'>('bootstrap')
const qrCodeCanvas = ref<HTMLCanvasElement | null>(null)
const usingFallback = ref<boolean>(false)
const copiedToClipboard = ref<boolean>(false)
const verificationPending = ref<boolean>(true)
let pollHandle: number | undefined

const parseRequestID = (uri: string): string => {
  try {
    const parsed = new URL(uri)
    const direct = parsed.searchParams.get('state') || parsed.searchParams.get('request_id')
    if (direct) {
      return direct
    }

    const requestJWT = parsed.searchParams.get('request')
    if (!requestJWT) {
      return ''
    }

    const [, payload] = requestJWT.split('.')
    if (!payload) {
      return ''
    }

    const normalized = payload.padEnd(payload.length + ((4 - payload.length % 4) % 4), '=')
    const claims = JSON.parse(atob(normalized.replace(/-/g, '+').replace(/_/g, '/')))
    return claims.state || claims.request_id || ''
  } catch {
    return ''
  }
}

const startPresentationPolling = (requestId: string) => {
  const poll = async () => {
    const status = await authenticationService.presentationStatus(requestId)
    if (!status.completed) {
      return
    }

    verificationPending.value = false
    if (pollHandle) {
      window.clearInterval(pollHandle)
      pollHandle = undefined
    }

    window.location.href = status.location || '/ui/auth/success'
  }

  // Poll immediately then every 2 seconds.
  void poll()
  pollHandle = window.setInterval(() => {
    void poll()
  }, 2000)
}

const renderQrCode = async (uri: string) => {
  // Wait for the canvas to exist because it is behind v-if="presentationUri".
  await nextTick()

  if (!qrCodeCanvas.value) {
    usingFallback.value = true
    return
  }

  await QRCode.toCanvas(qrCodeCanvas.value, uri, {
    width: 300,
    margin: 2,
    color: {
      dark: '#000000',
      light: '#FFFFFF',
    },
  })
}

const startPresentation = async () => {
  const loginChallenge = typeof route.query.login_challenge === 'string' ? route.query.login_challenge : undefined

  // No Hydra login_challenge means we entered this view directly (e.g. via
  // refresh or a deep link) rather than through Hydra's login redirect. In
  // that case bounce through /auth/login so Hydra issues a challenge and
  // redirects back here with `?login_challenge=...`. Without this, the
  // presentation flow completes locally but no OAuth session is ever
  // established and /auth/refresh has no cookie to exchange.
  if (!loginChallenge) {
    const loginUrl = await authenticationService.loginPath()
    if (loginUrl) {
      window.location.href = loginUrl
      return
    }
  }

  const uri = await authenticationService.presentationRequestUri(loginChallenge)
  if (!uri) {
    const loginUrl = await authenticationService.loginPath()
    if (loginUrl) {
      window.location.href = loginUrl
    }
    return
  }

  presentationUri.value = uri
  step.value = 'presentation'
  try {
    await renderQrCode(uri)
  } catch (err) {
    console.error('Failed to generate QR code:', err)
    usingFallback.value = true
  }

  const requestID = parseRequestID(uri)
  if (requestID) {
    startPresentationPolling(requestID)
  }
}

const requestBootstrapOffer = async () => {
  bootstrapError.value = ''
  const did = holderDid.value.trim()
  if (!did) {
    bootstrapError.value = 'Please enter your DID.'
    return
  }

  const res = await authenticationService.bootstrapAdminOffer(did)
  if (res.conflict) {
    await startPresentation()
    return
  }

  if (!res.offerUri) {
    bootstrapError.value = 'Could not create admin offer. Please try again.'
    return
  }

  offerUri.value = res.offerUri
  step.value = 'offer'
}

const continueToPresentation = async () => {
  const did = holderDid.value.trim()
  if (did) {
    await authenticationService.markBootstrapAdminClaimed(did)
  }
  await startPresentation()
}

onMounted(async () => {
  // Some OIDC providers may redirect to '/'.
  // In dem Fall direkt zu auth.success forwarden, ohne beforeEach zu involvieren.
  if (route.query.session_state && route.query.code && route.query.iss) {
    router.replace({ name: ROUTES.AUTH.SUCCESS, query: route.query })
    return
  }

  // Hydra redirects here with ?consent_challenge=... after login accept
  // (first-party DCS client). Forward to the backend's auto-accept endpoint
  // which will redirect to the OIDC callback to complete the code exchange.
  if (typeof route.query.consent_challenge === 'string' && route.query.consent_challenge) {
    window.location.href = `/api/auth/consent?consent_challenge=${encodeURIComponent(route.query.consent_challenge)}`
    return
  }

  const status = await authenticationService.bootstrapStatus()
  if (status.claimed) {
    await startPresentation()
    return
  }

  step.value = 'bootstrap'
})

onBeforeUnmount(() => {
  if (pollHandle) {
    window.clearInterval(pollHandle)
    pollHandle = undefined
  }
})

const copyToClipboard = () => {
  navigator.clipboard.writeText(offerUri.value || presentationUri.value)
  copiedToClipboard.value = true
  setTimeout(() => {
    copiedToClipboard.value = false
  }, 2000)
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200">
    <div class="card w-96 bg-base-100 shadow-xl">
      <div class="card-body">
        <h2 class="card-title text-center" v-if="step === 'bootstrap'">Register First Admin</h2>
        <h2 class="card-title text-center" v-else-if="step === 'offer'">Claim Admin Credential</h2>
        <h2 class="card-title text-center" v-else>Present Your Credential</h2>

        <div v-if="step === 'bootstrap'" class="flex flex-col gap-3">
          <p class="text-sm text-base-content/70">
            First-time setup: enter your wallet DID to generate the System Administrator credential offer.
          </p>
          <input
            v-model="holderDid"
            type="text"
            class="input input-bordered w-full"
            placeholder="did:web:example.org"
          />
          <p v-if="bootstrapError" class="text-error text-sm">{{ bootstrapError }}</p>
          <button @click="requestBootstrapOffer" class="btn btn-primary w-full">Generate Admin Offer</button>
        </div>

        <div v-else-if="step === 'offer'" class="flex flex-col items-center gap-4">
          <p class="text-sm text-center text-base-content/70">
            Claim this offer in your wallet, then continue to presentation login.
          </p>
          <textarea
            :value="offerUri"
            readonly
            class="textarea textarea-bordered w-full text-xs"
            rows="4"
          ></textarea>
          <div class="flex gap-2 w-full">
            <button @click="copyToClipboard" class="btn btn-sm btn-outline w-full">
              {{ copiedToClipboard ? 'Copied' : 'Copy Offer URI' }}
            </button>
            <button @click="continueToPresentation" class="btn btn-sm btn-primary w-full">
              I Claimed It
            </button>
          </div>
        </div>
        
        <div v-else-if="presentationUri" class="flex flex-col items-center gap-4">
          <!-- QR Code Display -->
          <div v-if="!usingFallback" class="border-2 border-base-300 p-2 rounded">
            <canvas ref="qrCodeCanvas"></canvas>
          </div>

          <!-- Fallback Text Display -->
          <div v-if="usingFallback" class="w-full">
            <p class="text-sm text-base-content/70 mb-2">Presentation URI:</p>
            <textarea
              :value="presentationUri"
              readonly
              class="textarea textarea-bordered w-full text-xs"
              rows="4"
            ></textarea>
          </div>

          <p class="text-sm text-center text-base-content/70">Copy and paste this URI into your wallet app</p>
          <p class="text-xs text-center text-base-content/60">
            {{ verificationPending ? 'Waiting for credential presentation...' : 'Verification complete, continuing sign-in...' }}
          </p>

          <div class="flex gap-2 w-full">
            <button
              @click="copyToClipboard"
              class="btn btn-sm btn-primary w-full"
            >
              {{ copiedToClipboard ? 'Copied' : 'Copy URI' }}
            </button>
          </div>
        </div>

        <!-- Loading State -->
        <div v-else class="flex items-center justify-center gap-2">
          <span class="loading loading-spinner loading-lg" />
          <span>Loading presentation request...</span>
        </div>
      </div>
    </div>
  </div>
</template>

