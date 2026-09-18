<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { sendLoginCode, login as loginRequest } from '@/api/auth'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
const router = useRouter()
const session = useSession()
const { invitation } = session
const email = ref(''); const code = ref(''); const error = ref('')
const busy = ref(false); const sent = ref(false); const seconds = ref(0)
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { if (seconds.value > 0) seconds.value-- }, 1000) })
onUnmounted(() => clearInterval(timer))
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await action() } catch (cause) { error.value = message(cause) } finally { busy.value = false }
}
function sendCode() { return run(async () => { await sendLoginCode(email.value, invitation.value); sent.value = true; seconds.value = 60 }) }
function login() { return run(async () => { await session.acceptUser(await loginRequest(email.value, code.value)); await router.replace(session.joinToken.value || !session.selected.value ? '/join' : '/') }) }
</script>

<template>
<section class="j-auth d-surface">
      <img src="/favicon.svg" width="48" height="48" alt="" /><p class="d-eyebrow">ONE MORE ROUND</p><h1>这桌，就等你了<span class="d-title-dot">。</span></h1><p>记下每一局的输赢与相聚。</p>
      <form @submit.prevent="sent ? login() : sendCode()">
        <label class="d-field">邮箱<input v-model="email" type="email" autocomplete="email" required :readonly="sent" /></label>
        <label v-if="!sent" class="d-field">试用邀请码（首次注册必填）<input v-model="invitation" autocomplete="off" /><small>已有账号无需填写。试用邀请与小组邀请分别使用。</small></label>
        <label v-if="sent" class="d-field">邮箱验证码<input v-model="code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" required /><small>10 分钟内有效，最多尝试 5 次。</small></label>
        <p v-if="error" class="j-error" role="alert">{{ error }}</p>
        <button class="d-button full" :disabled="busy">{{ busy ? '请稍候…' : sent ? '登录' : '发送验证码' }}</button>
        <div v-if="sent" class="j-actions"><button type="button" class="d-text-link" :disabled="busy || seconds > 0" @click="sendCode">{{ seconds ? `${seconds} 秒后可重发` : '重新发送' }}</button><button type="button" class="d-text-link" @click="sent = false; code = ''">更换邮箱</button></div>
      </form>
    </section>
</template>
