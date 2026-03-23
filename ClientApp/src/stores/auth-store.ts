import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'
import { useAuthTokenStore } from './auth-token-store'

type User = string | null

export const useAuthStore = defineStore('auth', () => {
  const authTokenStore = useAuthTokenStore()
  const user: Ref<User> = ref(null)

  const isAuthenticated = computed(() => !!user.value && authTokenStore.isAuthSet)

  function setUser(newUser: User) {
    user.value = newUser
  }

  function remove() {
    user.value = null
  }

  return { user, isAuthenticated, setUser, remove }
})
