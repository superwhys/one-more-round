<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import AccountBar from '@/components/layout/AccountBar.vue'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import '@/styles/theme.css'
import '@/styles/journal.css'
const session = useSession()
const { user, groups, selected, error } = session
const route = useRoute()
const router = useRouter()
const busy = ref(false)
const hasGroup = computed(() => !!route.meta.requiresGroup && !!selected.value)
async function switchGroup(id: string) {
  session.selectGroup(id)
  await router.push(id ? '/' : '/join')
}
async function logout() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await session.logout()
    await router.replace('/login')
  } catch (cause) {
    error.value = message(cause)
  } finally {
    busy.value = false
  }
}
async function retry() {
  await session.initialize(true)
  await router.replace(route.fullPath)
}
</script>

<template>
  <div class="journal-app journal-root">
    <a v-if="hasGroup" class="skip-link" href="#main-content">跳到主要内容</a>
    <AccountBar
      v-if="user"
      :user="user"
      :groups="groups"
      :selected="selected"
      :has-group="hasGroup"
      :back="!!route.meta.back"
      :busy="busy"
      @select="switchGroup"
      @setup="router.push('/join')"
      @logout="logout"
    />
    <p v-if="error" class="j-error j-global-error" role="alert">{{ error }} <button @click="retry">重新加载</button></p>
    <RouterView :key="`${user?.id ?? 'guest'}:${selected}`" />
  </div>
</template>
