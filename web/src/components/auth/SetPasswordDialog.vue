<script setup lang="ts">
import { ref } from 'vue'
import Modal from '@/components/common/AppModal.vue'
import { setPassword } from '@/api/auth'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import { validatePasswordRegistration } from '@/utils/password'
import type { User } from '@/types/journal'

const props = defineProps<{ user: User }>()
const emit = defineEmits<{ close: [] }>()
const session = useSession()
const username = ref(props.user.username ?? '')
const password = ref('')
const confirmation = ref('')
const error = ref('')
const busy = ref(false)
const saved = ref(false)

// close keeps an in-flight password change visible until its result is known.
function close() {
  if (!busy.value) emit('close')
}

// save adds password access to the current account without changing its identity or groups.
async function save() {
  if (busy.value || saved.value) return
  error.value = validatePasswordRegistration(username.value, password.value, confirmation.value)
  if (error.value) return
  busy.value = true
  try {
    session.user.value = await setPassword(username.value, password.value)
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
      <p role="status">密码已设置。下次可使用用户名{{ user.email ? '或已绑定邮箱' : '' }}登录。</p>
      <p class="d-note">当前浏览器保持登录，其他设备需要重新登录。</p>
      <button type="button" class="d-button full" @click="close">完成</button>
    </template>
    <form v-else @submit.prevent="save">
      <p class="d-note">为当前账号设置密码，小组与记录继续保留。设置成功后其他设备需要重新登录。</p>
      <label class="d-field"
        >用户名<input
          v-model="username"
          autocomplete="username"
          autocapitalize="none"
          :spellcheck="false"
          required
          :readonly="!!user.username"
          :disabled="busy"
        /><small>{{
          user.username ? '用户名已确定，不能修改。' : '3–64 个字符，区分大小写，不含空白或 @；设置后不能修改。'
        }}</small></label
      >
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
