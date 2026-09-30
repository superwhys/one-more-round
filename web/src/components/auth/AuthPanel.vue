<script setup lang="ts">
import { useInvitePreview } from '@/composables/useInvitePreview'
import { useSession } from '@/stores/session'
import favicon from '@/assets/favicon.svg'

withDefaults(defineProps<{ title: string; busy: boolean; cancelLabel?: string }>(), {
  cancelLabel: '暂不加入，返回普通登录',
})
const emit = defineEmits<{ cancel: [] }>()
const session = useSession()
const { joinToken } = session
const { preview, loading, error: inviteError, refresh } = useInvitePreview(joinToken)

// cancelInvite keeps the mailbox proof because admission is checked only at login.
function cancelInvite() {
  session.clearInvitations()
  emit('cancel')
}
</script>

<template>
  <section class="j-auth d-surface">
    <img :src="favicon" width="48" height="48" alt="" />
    <p class="d-eyebrow">ONE MORE ROUND</p>
    <h1>{{ title }}<span class="d-title-dot">。</span></h1>
    <template v-if="joinToken">
      <p v-if="loading" role="status">正在确认小组邀请…</p>
      <div v-else-if="preview" class="j-notice">
        <strong>你收到了一份小组邀请</strong>
        <p>加入「{{ preview.name }}」，一起记下每一局。</p>
        <p class="d-note">首次使用可注册并加入；已有账号登录后确认加入，无需另外填写邀请码。</p>
      </div>
      <p v-if="inviteError" class="j-error" role="alert">
        {{ inviteError }}，已有账号仍可继续登录。
        <button type="button" class="d-text-link" :disabled="busy" @click="refresh">重新检查</button>
      </p>
    </template>
    <p v-else>记下每一局的输赢与相聚。</p>
    <slot />
    <button v-if="joinToken" type="button" class="d-text-link" :disabled="busy" @click="cancelInvite">
      {{ cancelLabel }}
    </button>
  </section>
</template>
