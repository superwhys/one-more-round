<script setup lang="ts">
defineProps<{ loading?: boolean; error?: string; notice?: string }>()
defineEmits<{ retry: []; dismiss: [] }>()
</script>
<template>
  <p v-if="error" class="j-error" role="alert">{{ error }} <button type="button" @click="$emit('retry')">重新加载</button></p>
  <p v-if="notice" class="j-notice" role="status">{{ notice }}<button type="button" aria-label="关闭提示" @click="$emit('dismiss')">×</button></p>
  <p v-if="loading" role="status">正在翻开回忆…</p>
</template>
