<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Modal from '@/components/common/AppModal.vue'
import { getCurrentUser, setPassword } from '@/api/auth'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import { validatePassword, validatePasswordRegistration } from '@/utils/password'

const emit = defineEmits<{ close: [] }>()
const session = useSession()
const identifier = ref('')
const currentIdentifier = ref('')
const password = ref('')
const confirmation = ref('')
const error = ref('')
const busy = ref(false)
const saved = ref(false)
const loading = ref(true)
const loaded = ref(false)

// loadAccount reads the password binding independently of the account's display name.
async function loadAccount() {
  loading.value = true
  error.value = ''
  try {
    const account = await getCurrentUser()
    if (session.user.value?.id === account.id) session.user.value = account
    currentIdentifier.value = account.password_identifier ?? ''
    identifier.value = currentIdentifier.value
    loaded.value = true
  } catch (cause) {
    error.value = message(cause)
  } finally {
    loading.value = false
  }
}
onMounted(loadAccount)

// close keeps an in-flight password change visible until its result is known.
function close() {
  if (!busy.value) emit('close')
}

// save adds password access to the current account without changing its identity or groups.
async function save() {
  if (busy.value || saved.value || !loaded.value) return
  error.value = currentIdentifier.value
    ? validatePassword(password.value, confirmation.value)
    : validatePasswordRegistration(identifier.value, password.value, confirmation.value)
  if (error.value) return
  busy.value = true
  try {
    session.user.value = await setPassword(currentIdentifier.value ? undefined : identifier.value, password.value)
    identifier.value = session.user.value.password_identifier ?? identifier.value
    password.value = ''
    confirmation.value = ''
    saved.value = true
  } catch (cause) {
    error.value = message(cause)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal title="设置密码" @close="close">
    <template v-if="saved">
      <p role="status">密码已设置。下次请使用登录标识「{{ identifier }}」登录。</p>
      <p class="d-note">当前浏览器保持登录，其他设备需要重新登录。</p>
      <button type="button" class="d-button full" @click="close">完成</button>
    </template>
    <p v-else-if="loading" class="d-note" role="status">正在读取密码设置…</p>
    <template v-else-if="!loaded">
      <p class="j-error" role="alert">{{ error }}</p>
      <button type="button" class="d-button full" @click="loadAccount">重新读取</button>
    </template>
    <form v-else @submit.prevent="save">
      <p class="d-note">为当前账号设置密码，小组与记录继续保留。设置成功后其他设备需要重新登录。</p>
      <label class="d-field"
        >登录标识（用户名或邮箱）<input
          v-model="identifier"
          autocomplete="username"
          autocapitalize="none"
          :spellcheck="false"
          required
          :readonly="!!currentIdentifier"
          :disabled="busy"
        /><small>{{
          currentIdentifier
            ? '登录标识已确定，不能修改。'
            : '用户名为 3–64 个字符，区分大小写、不含空白；也可使用完整邮箱地址。设置后不能修改。'
        }}</small></label
      >
      <p class="d-note">
        账号展示名不用于密码登录。邮箱格式不会自动验证或绑定邮箱；已绑定邮箱也不会自动成为密码登录标识。
      </p>
      <label class="d-field"
        >新密码<input v-model="password" type="password" autocomplete="new-password" required :disabled="busy" /><small
          >8–128 个字符，可使用一句容易记住的长密码。</small
        ></label
      >
      <label class="d-field"
        >确认密码<input v-model="confirmation" type="password" autocomplete="new-password" required :disabled="busy"
      /></label>
      <p v-if="error" class="j-error" role="alert">{{ error }}</p>
      <button class="d-button full" :disabled="busy">{{ busy ? '正在保存…' : '保存密码' }}</button>
    </form>
  </Modal>
</template>
