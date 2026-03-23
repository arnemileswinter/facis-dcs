<template>
  <div class="flex-none">
    <label :for="pageSidebarId" class="btn btn-square btn-ghost lg:hidden">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="w-6 h-6 stroke-current">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16">
        </path>
      </svg>
    </label>

    <button @click="toggleSidebar()" class="btn btn-square btn-ghost hidden lg:inline-flex">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"
        :class="['w-6 h-6 transition-transform duration-300', isSidebarCollapsed ? 'rotate-180' : '']">
        <path stroke-linecap="round" stroke-linejoin="round" d="M18.75 19.5l-7.5-7.5 7.5-7.5m-6 15L5.25 12l7.5-7.5" />
      </svg>
    </button>
  </div>

  <div class="flex-1 px-4 font-bold text-lg">
    DCS <span class="font-normal opacity-50 ml-2">| Dashboard</span>
  </div>

  <div class="flex-none">
    <button @click="logout" class="btn btn-ghost btn-sm gap-2">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-4 h-4">
        <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 9V5.25A2.25 2.25 0 0 1 10.5 3h6a2.25 2.25 0 0 1 2.25 2.25v13.5A2.25 2.25 0 0 1 16.5 21h-6a2.25 2.25 0 0 1-2.25-2.25V15m-3 0-3-3m0 0 3-3m-3 3H15" />
      </svg>
      Log out
    </button>
  </div>
</template>

<script setup lang="ts">
import { usePageStore } from '@core/store/page'
import { storeToRefs } from 'pinia'

import { AuthenticationService } from '@/services/authentication-service'

const pageStore = usePageStore()
const { isSidebarCollapsed, pageSidebarId } = storeToRefs(pageStore)
const { toggleSidebar } = pageStore

function logout() {
  AuthenticationService.logout()
}
</script>
