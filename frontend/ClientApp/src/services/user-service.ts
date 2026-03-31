import http from '@/api/http'
import type { UserAllRequest, UserRolesByUserIdRequest } from '@/models/requests/user-request'
import type { UserAllResponse, UserRolesByUserIdResponse } from '@/models/responses/user-response'
import type { UserService } from '@/models/services/user-service'
import type { UserProfile } from '@/models/user'
import type { UserRole } from '@/types/user-role'
import type { AxiosRequestHeaders, AxiosResponse } from 'axios'
import { ref, type Ref } from 'vue'

const USER_BASE_URL = http.defaults.baseURL + '/users'

export const userService: UserService = {
  async getAllUsers(request?: UserAllRequest) {
    return Promise.resolve<AxiosResponse<UserAllResponse>>({
      data: { totalCount: users.value.length, items: users.value } as UserAllResponse,
      status: 200,
      statusText: 'OK',
      headers: {},
      config: {
        headers: {} as AxiosRequestHeaders,
      },
    }).then((res) => {
      console.log(res)
      return res.data.items
    })
    // return http
    //   .get<UserListResponse>('/users', { params: request, baseURL: USER_BASE_URL })
    //   .then((res) => res.data.items)
    //   .catch((err) => {
    //     console.error(err)
    //     return ({ totalCount: 0, items: [] } as UserListResponse).items
    //   })
  },

  async getRolesByUser(request: UserRolesByUserIdRequest) {
    return Promise.resolve<AxiosResponse<UserRolesByUserIdResponse>>({
      data: users.value.find((user) => user.id === request.userId)?.roleIds ?? [],
      status: 200,
      statusText: 'OK',
      headers: {},
      config: {
        headers: {} as AxiosRequestHeaders,
      },
    }).then((res) => {
      console.log(res)
      return res.data
    })
    // return http
    //   .get<UserRole[]>(`/users/${request.userId}/roles`, { baseURL: USER_BASE_URL })
    //   .then((res) => res.data)
    //   .catch((err) => {
    //     console.error(err)
    //     return [] as UserRole[]
    //   })
  },

  async getAuthorizedUsersWithRoles(...roles: [UserRole, ...UserRole[]]) {
    const allUsers = await this.getAllUsers()
    const authorizedUsers = await Promise.all(
      allUsers.map(async (user) => {
        const userRoles = await this.getRolesByUser({ userId: user.id })
        const isAuthorized = roles.some((role) => userRoles.includes(role))
        if (isAuthorized) {
          user.roleIds = roles.filter((role) => userRoles.includes(role))
        }
        return isAuthorized ? user : null
      }),
    )
    return authorizedUsers.filter((user) => user !== null)
  },
}

const mockUsers: UserProfile[] = [
  {
    participantId: 'part-000',
    firstName: 'Test',
    lastName: 'User',
    email: 'test@example.com',
    roleIds: ['TEMPLATE_APPROVER', 'TEMPLATE_CREATOR', 'TEMPLATE_MANAGER', 'TEMPLATE_REVIEWER', 'CONTRACT_CREATOR'],
    id: 'user-000',
    username: 'test',
  },
  {
    participantId: 'part-001',
    firstName: 'John',
    lastName: 'Doe',
    email: 'john.doe@example.com',
    roleIds: ['TEMPLATE_APPROVER', 'TEMPLATE_CREATOR', 'TEMPLATE_MANAGER', 'TEMPLATE_REVIEWER'],
    id: 'user-001',
    username: 'johndoe',
  },
  {
    participantId: 'part-002',
    firstName: 'Jane',
    lastName: 'Smith',
    email: 'jane.smith@example.com',
    roleIds: ['TEMPLATE_MANAGER', 'TEMPLATE_REVIEWER'],
    id: 'user-002',
    username: 'janesmith',
  },
  {
    participantId: 'part-003',
    firstName: 'Bob',
    lastName: 'Johnson',
    email: 'bob.johnson@example.com',
    roleIds: ['TEMPLATE_APPROVER', 'TEMPLATE_MANAGER'],
    id: 'user-003',
    username: 'bobjohnson',
  },
  {
    participantId: 'part-004',
    firstName: 'Alice',
    lastName: 'Williams',
    email: 'alice.williams@example.com',
    roleIds: ['TEMPLATE_REVIEWER'],
    id: 'user-004',
    username: 'alicewilliams',
  },
  {
    participantId: 'part-005',
    firstName: 'Charlie',
    lastName: 'Brown',
    email: 'charlie.brown@example.com',
    roleIds: ['TEMPLATE_CREATOR', 'TEMPLATE_REVIEWER'],
    id: 'user-005',
    username: 'charliebrown',
  },
  {
    participantId: 'part-006',
    firstName: 'Saoirse',
    lastName: 'Conrad',
    email: 'saoirse.conrad@example.com',
    roleIds: ['TEMPLATE_APPROVER', 'TEMPLATE_MANAGER'],
    id: 'user-006',
    username: 'saoirseconrad',
  },
]

export const users: Ref<UserProfile[]> = ref(mockUsers)
