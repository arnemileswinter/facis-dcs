import { useRouter } from 'vue-router'

export function useSyncPageTitle() {
  const router = useRouter()

  function setPageTitle() {
    const route = router.currentRoute.value
    document.title = (route.meta?.title as string) ?? 'DCS'
  }

  router.isReady().then(setPageTitle)
  router.afterEach(setPageTitle)
}