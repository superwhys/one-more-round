import { ref } from 'vue'
import { getCurrentUser, logout as logoutRequest } from '@/api/auth'
import { listGroups } from '@/api/group'
import { ApiError } from '@/api/request'
import { clearDrafts } from '@/utils/draft'
import { message } from '@/utils/error'
import type { Group, User } from '@/types/journal'

const user = ref<User | null>(null)
const groups = ref<Group[]>([])
const selected = ref('')
const error = ref('')
const invitation = ref('')
const joinToken = ref('')
let initialized = false
let initialization: Promise<void> | undefined

function selectGroup(id: string) {
  selected.value = id
  if (user.value) {
    try { localStorage.setItem(`omr:group:${user.value.id}`, id) } catch { /* optional preference */ }
  }
}
async function loadGroups() {
  groups.value = await listGroups()
  if (!groups.value.some(group => group.id === selected.value)) selectGroup(groups.value[0]?.id ?? '')
}
async function acceptUser(account: User) {
  error.value = ''
  user.value = account
  try { selected.value = localStorage.getItem(`omr:group:${account.id}`) ?? '' } catch { selected.value = '' }
  await loadGroups()
}
function expire() {
  user.value = null
  groups.value = []
  selected.value = ''
  error.value = '登录已失效，请重新登录'
}
async function initialize(force = false) {
  if (initialization) return initialization
  if (initialized && !force) return
  initialization = (async () => {
    error.value = ''
    try { await acceptUser(await getCurrentUser()) }
    catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) { user.value = null; groups.value = []; selected.value = '' }
      else error.value = message(cause)
    } finally { initialized = true }
  })()
  try { await initialization } finally { initialization = undefined }
}
async function logout() {
  await logoutRequest()
  try { clearDrafts() } catch { error.value = '已退出登录，但浏览器未能清除本地草稿' }
  user.value = null
  groups.value = []
  selected.value = ''
}

export function useSession() {
  return { user, groups, selected, error, invitation, joinToken, initialize, acceptUser, selectGroup, loadGroups, expire, logout }
}
