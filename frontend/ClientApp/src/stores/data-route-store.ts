import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { RouteMap } from 'vue-router'

export const useDataRouteStore = defineStore('dataRoute', () => {
  const loadedRoutes = ref(new Set<keyof RouteMap>())

  function isRouteDataLoaded(routeName: keyof RouteMap | undefined) {
    return routeName ? loadedRoutes.value.has(routeName) : false
  }

  function addDataRouteLoaded(routeName: keyof RouteMap | undefined) {
    if (routeName !== undefined) {
      loadedRoutes.value.add(routeName)
    }
  }

  return { loadedRoutes, isRouteDataLoaded, addDataRouteLoaded }
})
