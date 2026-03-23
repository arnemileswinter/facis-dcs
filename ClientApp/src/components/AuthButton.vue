<script setup lang="ts">
import { AuthenticationService } from '@/services/authentication-service'
import { useAuthStore } from '@/stores/auth-store'
import { storeToRefs } from 'pinia'
import { onMounted, ref } from 'vue'

const authStore = useAuthStore()
const { isAuthenticated } = storeToRefs(authStore)

const loginPath = ref('')

onMounted(async () => {
  loginPath.value = await AuthenticationService.getLoginPath()
})

function logout() {
  AuthenticationService.logout()
}
</script>

<template>
  <div>
    <a v-if="!isAuthenticated" :href="loginPath" class="btn btn-block btn-accent flex-1 text-center">Single Sign On</a>
    <button v-else class="btn btn-block btn-accent flex-1 text-center" @click="logout">Logout</button>
  </div>
</template>
