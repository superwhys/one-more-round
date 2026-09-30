<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { RouterLink } from 'vue-router'
import { sendLoginCode, login as loginRequest, loginPassword } from '@/api/auth'
import AuthPanel from '@/components/auth/AuthPanel.vue'
import { useAuthCompletion } from '@/composables/useAuthCompletion'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
const completeLogin = useAuthCompletion()
const session = useSession()
const { invitation, joinToken } = session
const method = ref<'email' | 'password'>('email')
const identifier = ref('')
const password = ref('')
const email = ref('')
const code = ref('')
const error = ref('')
const busy = ref(false)
const sent = ref(false)
const seconds = ref(0)
// Save only the address and send time, never the verification code.
const loginKey = 'omr:pending-login'
try {
  const saved: unknown = JSON.parse(sessionStorage.getItem(loginKey) ?? 'null')
  if (
    saved &&
    typeof saved === 'object' &&
    'email' in saved &&
    typeof saved.email === 'string' &&
    'sentAt' in saved &&
    typeof saved.sentAt === 'number'
  ) {
    const elapsed = Date.now() - saved.sentAt
    if (elapsed >= 0 && elapsed < 10 * 60 * 1000) {
      email.value = saved.email
      sent.value = true
      seconds.value = Math.max(0, Math.ceil(60 - elapsed / 1000))
    }
  }
} catch {
  /* optional recovery */
}
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  timer = setInterval(() => {
    if (seconds.value > 0) seconds.value--
  }, 1000)
})
onUnmounted(() => clearInterval(timer))
watch(joinToken, () => {
  error.value = ''
})
// clearLogin discards only the optional mailbox recovery state.
function clearLogin() {
  try {
    sessionStorage.removeItem(loginKey)
  } catch {
    /* optional recovery */
  }
}
// changeEmail starts a new mailbox proof without discarding the invitation.
function changeEmail() {
  sent.value = false
  code.value = ''
  clearLogin()
}
// changeMethod clears secrets when switching between authentication methods.
function changeMethod(next: 'email' | 'password') {
  method.value = next
  password.value = ''
  code.value = ''
  error.value = ''
}
// run prevents concurrent submissions and leaves failed form inputs available for retry.
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await action()
  } catch (cause) {
    error.value = message(cause)
  } finally {
    busy.value = false
  }
}
// sendCode preserves the address and cooldown for refresh recovery.
function sendCode() {
  return run(async () => {
    await sendLoginCode(email.value)
    sent.value = true
    seconds.value = 60
    try {
      sessionStorage.setItem(loginKey, JSON.stringify({ email: email.value, sentAt: Date.now() }))
    } catch {
      /* optional recovery */
    }
  })
}
// login exchanges the selected proof for the existing account and group flow.
function login() {
  return run(async () => {
    const result =
      method.value === 'password'
        ? await loginPassword(identifier.value, password.value)
        : await loginRequest(email.value, code.value, invitation.value, joinToken.value)
    password.value = ''
    code.value = ''
    await completeLogin(result)
  })
}
</script>

<template>
  <AuthPanel title="这桌，就等你了" :busy="busy" @cancel="error = ''">
    <div class="d-segment auth-method" role="group" aria-label="登录方式">
      <button
        type="button"
        :class="{ active: method === 'email' }"
        :aria-pressed="method === 'email'"
        :disabled="busy"
        @click="changeMethod('email')"
      >
        邮箱验证码
      </button>
      <button
        type="button"
        :class="{ active: method === 'password' }"
        :aria-pressed="method === 'password'"
        :disabled="busy"
        @click="changeMethod('password')"
      >
        账号密码
      </button>
    </div>
    <form @submit.prevent="method === 'password' || sent ? login() : sendCode()">
      <template v-if="method === 'password'">
        <label class="d-field"
          >用户名或邮箱<input
            v-model="identifier"
            autocomplete="username"
            autocapitalize="none"
            :spellcheck="false"
            required
            :disabled="busy"
        /></label>
        <label class="d-field"
          >密码<input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            :disabled="busy"
          /><small>已有邮箱账号可先用验证码登录，在「我的账号」中设置密码。</small></label
        >
      </template>
      <template v-else>
        <label class="d-field"
          >邮箱<input v-model="email" type="email" autocomplete="email" required :readonly="sent || busy"
        /></label>
        <label v-if="!joinToken" class="d-field"
          >试用邀请码（首次注册必填）<input v-model="invitation" autocomplete="off" :disabled="busy" /><small
            >已有账号无需填写。朋友邀请你加入小组时，直接打开小组邀请链接即可。</small
          ></label
        >
        <label v-if="sent" class="d-field"
          >邮箱验证码<input
            v-model="code"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="6"
            required
            :disabled="busy"
          /><small>10 分钟内有效，最多尝试 5 次。</small></label
        >
      </template>
      <p v-if="error" class="j-error" role="alert">{{ error }}</p>
      <button class="d-button full" :disabled="busy">
        {{ busy ? '请稍候…' : method === 'password' || sent ? '登录' : '发送验证码' }}
      </button>
      <div v-if="method === 'email' && sent" class="j-actions">
        <button type="button" class="d-text-link" :disabled="busy || seconds > 0" @click="sendCode">
          {{ seconds ? `${seconds} 秒后可重发` : '重新发送' }}</button
        ><button type="button" class="d-text-link" :disabled="busy" @click="changeEmail">更换邮箱</button>
      </div>
    </form>
    <p class="d-note auth-register">
      第一次来？<RouterLink
        to="/register"
        class="d-text-link"
        :aria-disabled="busy"
        @click="busy && $event.preventDefault()"
        >用账号密码注册</RouterLink
      >
    </p>
  </AuthPanel>
</template>

<style scoped>
.auth-method {
  margin-top: 24px;
}
.auth-register {
  margin: 24px 0;
}
</style>
