<script setup lang="ts">
import { AuthenticationService } from '@/services/authentication-service'
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

onMounted(async () => {
  // Exchange refresh token cookie for access token
  await AuthenticationService.refresh()
  // Redirect to templates list on success
  router.replace({ name: 'templates.list' })
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200">
    <span class="loading loading-spinner loading-lg" />
  </div>
</template>
