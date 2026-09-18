<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getServiceStatus } from '@/api/status'

const state = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')

async function checkConnection() {
  state.value = 'loading'
  try {
    await getServiceStatus()
    state.value = 'ready'
  } catch (error) {
    state.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : '暂时无法连接'
  }
}

onMounted(checkConnection)
</script>

<template>
  <div class="connection" role="status" aria-live="polite">
    <span class="connection-dot" :class="state" aria-hidden="true"></span>
    <span v-if="state === 'loading'">正在连接…</span>
    <span v-else-if="state === 'ready'">连接正常</span>
    <template v-else>
      <span>{{ errorMessage }}</span>
      <button class="text-button" type="button" @click="checkConnection">重试</button>
    </template>
  </div>
</template>
