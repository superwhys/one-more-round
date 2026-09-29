<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { sendLoginCode, login as loginRequest } from '@/api/auth'
import { useInvitePreview } from '@/composables/useInvitePreview'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import favicon from '@/assets/favicon.svg'
const router = useRouter()
const session = useSession()
const { invitation, joinToken } = session
const { preview, loading, error: inviteError, refresh } = useInvitePreview(joinToken)
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
function clearLogin() {
  try {
    sessionStorage.removeItem(loginKey)
  } catch {
    /* optional recovery */
  }
}
function changeEmail() {
  sent.value = false
  code.value = ''
  clearLogin()
}
// cancelInvite keeps the mailbox proof because admission is checked only at login.
function cancelInvite() {
  session.clearInvitations()
  error.value = ''
}
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
function login() {
  return run(async () => {
    const result = await loginRequest(email.value, code.value, invitation.value, joinToken.value)
    invitation.value = ''
    if (result.group_id) joinToken.value = ''
    clearLogin()
    // Authentication has committed. Existing accounts confirm a pending group
    // invitation after login, even when its preview is currently unavailable.
    try {
      await session.acceptUser(result)
    } catch {
      session.error.value = '已登录，但小组列表加载失败，请重新加载'
    }
    if (result.group_id) session.selectGroup(result.group_id)
    await router.replace(result.group_id ? '/group' : joinToken.value || !session.selected.value ? '/join' : '/')
  })
}
</script>

<template>
  <section class="j-auth d-surface">
    <img :src="favicon" width="48" height="48" alt="" />
    <p class="d-eyebrow">ONE MORE ROUND</p>
    <h1>这桌，就等你了<span class="d-title-dot">。</span></h1>
    <template v-if="joinToken">
      <p v-if="loading" role="status">正在确认小组邀请…</p>
      <div v-else-if="preview" class="j-notice">
        <strong>你收到了一份小组邀请</strong>
        <p>加入「{{ preview.name }}」，一起记下每一局。</p>
        <p class="d-note">首次使用验证邮箱即可注册并加入；已有账号登录后确认加入，无需另外填写邀请码。</p>
      </div>
      <p v-if="inviteError" class="j-error" role="alert">
        {{ inviteError }}，已有账号仍可继续登录。
        <button type="button" class="d-text-link" @click="refresh">重新检查</button>
      </p>
    </template>
    <p v-else>记下每一局的输赢与相聚。</p>
    <form @submit.prevent="sent ? login() : sendCode()">
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
        /><small>10 分钟内有效，最多尝试 5 次。</small></label
      >
      <p v-if="error" class="j-error" role="alert">{{ error }}</p>
      <button class="d-button full" :disabled="busy">
        {{ busy ? '请稍候…' : sent ? '登录' : '发送验证码' }}
      </button>
      <div v-if="sent" class="j-actions">
        <button type="button" class="d-text-link" :disabled="busy || seconds > 0" @click="sendCode">
          {{ seconds ? `${seconds} 秒后可重发` : '重新发送' }}</button
        ><button type="button" class="d-text-link" :disabled="busy" @click="changeEmail">更换邮箱</button>
      </div>
    </form>
    <button v-if="joinToken" type="button" class="d-text-link" :disabled="busy" @click="cancelInvite">
      暂不加入，返回普通登录
    </button>
  </section>
</template>
