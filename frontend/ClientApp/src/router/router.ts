import { getUIBasePath } from '@/config'
import ApproveContractTemplateView from '@/modules/template-repository/views/ApproveContractTemplateView.vue'
import ReviewContractTemplateView from '@/modules/template-repository/views/ReviewContractTemplateView.vue'
import ViewContractTemplateView from '@/modules/template-repository/views/ViewContractTemplateView.vue'
import { authenticationService } from '@/services/authentication-service'
import { useAuthStore } from '@/stores/auth-store'
import { useDataRouteStore } from '@/stores/data-route-store'
import AuthSuccessView from '@/views/auth/AuthSuccessView.vue'
import LoginView from '@/views/auth/LoginView.vue'
import ContractListView from '@/views/contract-list/ContractListView.vue'
import NewContractView from '@/views/contract-list/NewContractView.vue'
import ContractTemplateListView from '@/views/contract-template-list/ContractTemplateListView.vue'
import ContractTemplateTaskView from '@/views/contract-template-list/ContractTemplateTaskView.vue'
import TemplateCatalogueListView from '@/views/template-repository/TemplateCatalogueListView.vue'
import TemplateCatalogueView from '@/views/template-repository/TemplateCatalogueView.vue'
import TemplateCatalogueAdminView from '@/views/template-repository/TemplateCatalogueAdminView.vue'
import { DocumentCheckIcon, DocumentDuplicateIcon, DocumentMagnifyingGlassIcon, DocumentTextIcon } from '@heroicons/vue/20/solid'
import NewContractTemplateView from '@template-repository/views/NewContractTemplateView.vue'
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const ROUTES = {
  HOME: 'home',
  TEMPLATES: {
    LIST: 'templates.list',
    NEW: 'templates.new',
    EDIT: 'templates.edit',
    VIEW: 'templates.view',
    REVIEW: 'templates.review',
    APPROVE: 'templates.approve',
    TASKS: {
      REVIEW: 'templates.tasks.review',
      APPROVAL: 'templates.tasks.approve',
    },
  },
  TEMPLATE_CATALOGUES: {
    LIST: 'template.catalogues.list',
    VIEW: 'template.catalogues.view',
    ADMIN: 'template.catalogues.admin',
  },
  AUTH: {
    SUCCESS: 'auth.success',
  },
  CONTRACTS: {
    LIST: 'contracts.list',
    NEW: 'contracts.new',
    EDIT: 'contracts.edit',
  },
} as const

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: ROUTES.HOME,
    meta: { name: 'DCS', hideInSidebar: true, requiresAuth: false, layout: 'blank', title: 'DCS' },
    component: LoginView,
  },
  {
    path: '/templates',
    name: ROUTES.TEMPLATES.LIST,
    component: ContractTemplateListView,
    meta: {
      name: 'Contract Templates',
      icon: DocumentTextIcon,
      requiresAuth: true,
      title: 'DCS - Templates',
      order: 1,
    },
  },
  {
    path: '/templates/new',
    name: ROUTES.TEMPLATES.NEW,
    component: NewContractTemplateView,
    meta: {
      name: 'New Template',
      hideInSidebar: true,
      requiresAuth: true,
      title: 'DCS - New Template',
      roles: ['TEMPLATE_CREATOR'],
    },
  },
  {
    path: '/templates/edit/:did',
    name: ROUTES.TEMPLATES.EDIT,
    component: NewContractTemplateView,
    meta: {
      name: 'Edit Template',
      hideInSidebar: true,
      requiresAuth: true,
      title: 'DCS - Edit Template',
      roles: ['TEMPLATE_CREATOR', 'TEMPLATE_REVIEWER'],
    },
  },
  {
    path: '/templates/view/:did',
    name: ROUTES.TEMPLATES.VIEW,
    component: ViewContractTemplateView,
    meta: { name: 'View Template', hideInSidebar: true, requiresAuth: true, title: 'DCS - View Template' },
  },
  {
    path: '/templates/review/:did',
    name: ROUTES.TEMPLATES.REVIEW,
    component: ReviewContractTemplateView,
    meta: {
      name: 'Review Template',
      hideInSidebar: true,
      requiresAuth: true,
      title: 'DCS - Review Template',
      roles: ['TEMPLATE_REVIEWER'],
    },
  },
  {
    path: '/templates/approve/:did',
    name: ROUTES.TEMPLATES.APPROVE,
    component: ApproveContractTemplateView,
    meta: {
      name: 'Approve Template',
      hideInSidebar: true,
      requiresAuth: true,
      title: 'DCS - Approve Template',
      roles: ['TEMPLATE_APPROVER'],
    },
  },
  {
    path: '/templates/tasks/review',
    name: ROUTES.TEMPLATES.TASKS.REVIEW,
    component: ContractTemplateTaskView,
    meta: {
      name: 'Assigned Review Tasks',
      icon: DocumentMagnifyingGlassIcon,
      requiresAuth: true,
      title: 'DCS - Review Tasks',
      order: 3.1,
      roles: ['TEMPLATE_REVIEWER'],
      requiresData: true,
    },
    beforeEnter: (to) => {
      const dataRouteStore = useDataRouteStore()
      if (!dataRouteStore.isRouteDataLoaded(to.name)) {
        return { name: ROUTES.TEMPLATES.LIST }
      }
    },
  },
  {
    path: '/templates/tasks/approve',
    name: ROUTES.TEMPLATES.TASKS.APPROVAL,
    component: ContractTemplateTaskView,
    meta: {
      name: 'Assigned Approval Tasks',
      icon: DocumentCheckIcon,
      requiresAuth: true,
      title: 'DCS - Approval Tasks',
      order: 3.2,
      roles: ['TEMPLATE_APPROVER'],
      requiresData: true,
    },
    beforeEnter: (to) => {
      const dataRouteStore = useDataRouteStore()
      if (!dataRouteStore.isRouteDataLoaded(to.name)) {
        return { name: ROUTES.TEMPLATES.LIST }
      }
    },
  },
  {
    path: '/catalogues',
    name: ROUTES.TEMPLATE_CATALOGUES.LIST,
    component: TemplateCatalogueListView,
    meta: {
      name: 'Template Catalogues',
      icon: DocumentTextIcon,
      requiresAuth: true,
      title: 'DCS - Template Catalogues',
      order: 4,
    },
  },
  {
    path: '/catalogues/:did',
    name: ROUTES.TEMPLATE_CATALOGUES.VIEW,
    component: TemplateCatalogueView,
    meta: {
      name: 'Template Catalogue',
      hideInSidebar: true,
      requiresAuth: true,
      title: 'DCS - Template Catalogue',
    },
  },
  {
    path: '/catalogues/admin',
    name: ROUTES.TEMPLATE_CATALOGUES.ADMIN,
    component: TemplateCatalogueAdminView,
    meta: {
      name: 'Template Catalogue Admin',
      icon: DocumentTextIcon,
      requiresAuth: true,
      title: 'DCS - Template Catalogue Admin',
      order: 5,
      roles: ['SYSTEM_ADMINISTRATOR'],
    },
  },
  {
    path: '/contracts',
    name: ROUTES.CONTRACTS.LIST,
    component: ContractListView,
    meta: {
      name: 'Contracts',
      icon: DocumentDuplicateIcon,
      requiresAuth: true,
      title: 'DCS - Contracts',
      order: 2,
      roles: ['CONTRACT_CREATOR', 'CONTRACT_REVIEWER', 'CONTRACT_APPROVER', 'CONTRACT_MANAGER']
    },
  },
  {
    path: '/contracts/new',
    name: ROUTES.CONTRACTS.NEW,
    component: NewContractView,
    meta: {
      name: 'New Contract',
      hideInSidebar: true,
      requiresAuth: true,
      title: 'DCS - New Contract',
      roles: ['CONTRACT_CREATOR'],
    },
  },
  {
    path: '/contracts/edit/:did',
    name: ROUTES.CONTRACTS.EDIT,
    component: NewContractView,
    meta: {
      name: 'Edit Contract',
      hideInSidebar: true,
      requiresAuth: true,
      title: 'DCS - Edit Contract',
      roles: ['CONTRACT_CREATOR'],
    },
  },
  {
    path: '/auth/success',
    name: ROUTES.AUTH.SUCCESS,
    meta: { hideInSidebar: true, requiresAuth: false, layout: 'blank', title: 'DCS - Auth Success' },
    component: AuthSuccessView,
  },
]

const router = createRouter({
  history: createWebHistory(getUIBasePath()),
  routes: routes,
})

router.beforeEach(async (to) => {
  if (to.meta.requiresAuth === false) {
    return true
  }

  const authStore = useAuthStore()
  if (authStore.isAuthenticated) {
    return true
  }

  await authenticationService.refresh()
  if (authStore.isAuthenticated) {
    return true
  }

  const loginUrl = await authenticationService.loginPath()
  if (loginUrl) {
    window.location.href = loginUrl
    return false
  }

  return { name: ROUTES.HOME }
})

router.beforeEach((to) => {
  if (!to.meta.roles) {
    return true
  }
  const authStore = useAuthStore()
  const hasAuthorizedRole = authStore.user?.roles?.some((role) => to.meta.roles?.includes(role)) ?? false
  if (!hasAuthorizedRole) {
    return { name: ROUTES.HOME }
  }
})

export { router, ROUTES }
