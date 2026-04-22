import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'url'
import { defineConfig, loadEnv, type Plugin } from 'vite'

// https://vite.dev/config/
export default defineConfig(({ mode, command }) => {
  const env = loadEnv(mode, process.cwd(), 'DCS_')
  const basePath = env.DCS_UI_PATH || '/ui/'
  const apiTarget = env.DCS_API_TARGET || 'http://localhost:8991'
  const ocmwWellKnownTarget = env.DCS_OCMW_WELLKNOWN_TARGET || 'http://localhost:30182'
  const ocmwTokenTarget = env.DCS_OCMW_TOKEN_TARGET || 'http://localhost:31803'
  const ocmwIssuerTarget = env.DCS_OCMW_ISSUER_TARGET || 'http://localhost:30180'
  const ocmwCvTarget = env.DCS_OCMW_CV_TARGET || 'http://localhost:32035'

  // Local dev OID4VCI topology:
  // - Well-known metadata: well-known-service NodePort
  // - Token endpoint: pre-authorization-bridge NodePort
  // - Credential issuance: issuance-service NodePort
  // - Presentation proof callback (/api/presentation/proof/...) is proxied to
  //   the CV NodePort separately above so it matches before the generic /api.
  const oid4vciProxy = {
    '^/v1/.*/\\.well-known/': {
      target: ocmwWellKnownTarget,
      changeOrigin: true
    },
    '^/v1/.*/token$': {
      target: ocmwTokenTarget,
      changeOrigin: true,
      rewrite: () => '/token'
    },
    '^/v1': {
      target: ocmwIssuerTarget,
      changeOrigin: true
    }
  }

  
  // Plugin to inject base href in dev mode
  const baseHrefPlugin: Plugin = {
    name: 'base-href-inject',
    transformIndexHtml: {
      order: 'pre',
      handler(html) {
        if (command === 'serve') {
          // In dev mode, replace the placeholder with the actual base path
          return html.replace('__DCS_UI_BASE_PATH__', basePath)
        }
        // In build mode, leave the placeholder for inject-config.sh to handle
        return html
      }
    }
  }

  return {
    // during build, use relative paths such that we respect <base href>
    base: command === 'build' ? './' : basePath,
    plugins: [baseHrefPlugin, vue(), tailwindcss()],
    envPrefix: 'DCS_',
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src/', import.meta.url)),
        '@core': fileURLToPath(new URL('./src/core/', import.meta.url)),
        '@template-repository': fileURLToPath(new URL('./src/modules/template-repository/', import.meta.url)),
      },
    },
    server: {
      proxy: {
        '/.well-known': {
          target: 'http://localhost:30080',
          changeOrigin: true
        },
        '/oauth2': {
          target: 'http://localhost:30080',
          changeOrigin: true
        },
        // CV proof callback must be matched before the generic /api route below.
        // CV emits response_uri using its publicBasePath default
        // (`/api/presentation/proof`), but the actual receive handler is mounted
        // at `/v1/tenants/{tenantId}/presentation/proof/{id}`. Rewrite the path
        // when proxying so the wallet's POST lands on the correct route.
        '^/api/presentation/proof/': {
          target: ocmwCvTarget,
          changeOrigin: true,
          rewrite: (p: string) => p.replace(
            /^\/api\/presentation\/proof\//,
            '/v1/tenants/tenant_space/presentation/proof/'
          )
        },
        '/api': {
          target: apiTarget,
          changeOrigin: true
        },
        '/hydra': {
          target: apiTarget,
          changeOrigin: true
        },
        ...oid4vciProxy,
      },
    },
  }
})
