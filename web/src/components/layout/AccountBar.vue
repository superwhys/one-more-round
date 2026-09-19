<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import type { Group, User } from '@/types/journal'
const props = defineProps<{ user: User; groups: Group[]; selected: string; hasGroup: boolean; back: boolean; busy: boolean }>()
defineEmits<{ select: [id: string]; setup: []; logout: [] }>()
const route = useRoute()
const accountOpen = ref(false)
const account = ref<HTMLElement>()
const trigger = ref<HTMLButtonElement>()
const panelID = useId()
const initial = computed(() => Array.from(props.user.email)[0]?.toUpperCase() ?? '我')
const groupName = computed(() => props.groups.find(group => group.id === props.selected)?.name ?? '创建 / 加入小组')
function dismiss(event: Event) {
  if (event.target instanceof Node && !account.value?.contains(event.target)) accountOpen.value = false
}
function escape(event: KeyboardEvent) {
  if (event.key !== 'Escape' || !accountOpen.value) return
  event.preventDefault()
  accountOpen.value = false
  trigger.value?.focus()
}
watch(() => route.fullPath, () => { accountOpen.value = false })
onMounted(() => {
  document.addEventListener('pointerdown', dismiss)
  document.addEventListener('focusin', dismiss)
  document.addEventListener('keydown', escape)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', dismiss)
  document.removeEventListener('focusin', dismiss)
  document.removeEventListener('keydown', escape)
})
</script>

<template>
<header class="j-account" :class="{ 'has-group': hasGroup }">
  <div class="j-header-context">
    <RouterLink v-if="back" to="/" class="j-header-back" aria-label="回到回顾"><Icon name="back" /><span>回到回顾</span></RouterLink>
    <RouterLink v-else to="/" class="j-header-brand" aria-label="又一局 · 回到首页"><img src="/favicon.svg" alt="" /><span v-if="!groups.length">又一局</span></RouterLink>
    <label v-if="groups.length" class="j-header-group"><span>当前小组</span><select class="j-group-select" :value="selected" :title="groupName" aria-label="当前小组" @change="$emit('select', ($event.target as HTMLSelectElement).value)"><component is="button" type="button" class="j-group-button">{{ groupName }}</component><option value="">创建 / 加入小组</option><option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option></select></label>
  </div>
  <span v-if="hasGroup" class="j-header-private"><Icon name="lock" :size="14" />小组私有<span>· 邀请试用</span></span>
  <div ref="account" class="j-account-menu">
    <button ref="trigger" type="button" class="j-account-trigger" aria-label="我的账号" :aria-expanded="accountOpen" :aria-controls="panelID" @click="accountOpen = !accountOpen"><span class="j-account-avatar" aria-hidden="true">{{ initial }}</span><span class="j-account-label">我的账号</span><Icon name="down" :size="14" /></button>
    <div v-if="accountOpen" :id="panelID" class="j-account-panel" role="region" aria-label="账号选项">
      <div class="j-account-identity"><span>当前账号</span><strong>{{ user.email }}</strong><small>邀请试用 · 记下每一次相聚</small></div>
      <button type="button" class="j-account-action" @click="accountOpen = false; $emit('setup')"><Icon name="plus" :size="18" />创建 / 加入小组<Icon name="arrow" :size="16" /></button>
      <div class="j-account-signout"><button type="button" class="j-account-action" :disabled="busy" @click="$emit('logout')"><Icon name="logout" :size="18" />{{ busy ? '正在退出…' : '退出登录' }}</button></div>
    </div>
  </div>
</header>
</template>
