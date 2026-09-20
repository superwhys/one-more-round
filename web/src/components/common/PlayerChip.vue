<script setup lang="ts">
import { computed } from 'vue'

// 玩家标识：头像取昵称首字符并大写，随后是昵称与「我」标记，使用方通过默认插槽补充分数、选中态等内容
const props = withDefaults(defineProps<{ name: string; me?: boolean; subtitle?: string; stacked?: boolean }>(), {
  me: false,
  subtitle: '',
  stacked: false,
})
const initial = computed(() => Array.from(props.name.trim())[0]?.toUpperCase() ?? '玩')
</script>

<template>
  <span class="d-player-chip" :class="{ stacked }">
    <span class="d-avatar" aria-hidden="true">{{ initial }}</span>
    <span class="d-player-label"
      >{{ name }}{{ me ? '（我）' : '' }}<small v-if="subtitle">{{ subtitle }}</small></span
    >
    <slot />
  </span>
</template>
