<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { registerPassword } from '@/api/auth'
import AuthPanel from '@/components/auth/AuthPanel.vue'
import { useAuthCompletion } from '@/composables/useAuthCompletion'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import { validatePasswordRegistration } from '@/utils/password'

const { invitation, joinToken } = useSession()
const completeLogin = useAuthCompletion()
const username = ref('')
const password = ref('')
const confirmation = ref('')
const error = ref('')
const busy = ref(false)
watch(joinToken, () => {
  error.value = ''
})

// register requires an invitation and keeps failed inputs available for correction.
async function register() {
  if (busy.value) return
  error.value = validatePasswordRegistration(username.value, password.value, confirmation.value)
  if (!error.value && !joinToken.value && !invitation.value.trim())
    error.value = '首次注册需要有效的试用邀请码或小组邀请'
  if (error.value) return
  busy.value = true
  try {
    const result = await registerPassword({
      username: username.value,
      password: password.value,
      invite: invitation.value,
      group_token: joinToken.value,
    })
    password.value = ''
    confirmation.value = ''
    await completeLogin(result)
  } catch (cause) {
    error.value = message(cause)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthPanel title="为下一局，留个名字" :busy="busy" cancel-label="暂不加入，改用试用邀请码" @cancel="error = ''">
    <form @submit.prevent="register">
      <label class="d-field"
        >用户名<input
          v-model="username"
          autocomplete="username"
          autocapitalize="none"
          :spellcheck="false"
          required
          :disabled="busy"
        /><small>3–64 个字符，区分大小写，不含空白或 @。</small></label
      >
      <label class="d-field"
        >密码<input v-model="password" type="password" autocomplete="new-password" required :disabled="busy" /><small
          >8–128 个字符，可使用一句容易记住的长密码。</small
        ></label
      >
      <label class="d-field"
        >确认密码<input v-model="confirmation" type="password" autocomplete="new-password" required :disabled="busy"
      /></label>
      <label v-if="!joinToken" class="d-field"
        >试用邀请码<input v-model="invitation" autocomplete="off" required :disabled="busy" /><small
          >也可以打开朋友分享的小组邀请链接后注册。</small
        ></label
      >
      <p v-if="error" class="j-error" role="alert">{{ error }}</p>
      <button class="d-button full" :disabled="busy">{{ busy ? '正在注册…' : '注册并登录' }}</button>
    </form>
    <p class="d-note auth-login">已有邮箱或微信账号？请先登录原账号，在「我的账号」中设置密码。</p>
    <RouterLink to="/login" class="d-text-link" :aria-disabled="busy" @click="busy && $event.preventDefault()"
      >返回登录</RouterLink
    >
  </AuthPanel>
</template>

<style scoped>
.auth-login {
  margin: 24px 0 12px;
}
</style>
