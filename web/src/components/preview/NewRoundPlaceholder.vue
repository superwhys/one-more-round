<script setup lang="ts">
import { RouterLink } from 'vue-router'
import EmptyState from '@/components/common/EmptyState.vue'
</script>

<template>
  <div class="page-heading"><p class="eyebrow">又一局，又一个故事</p><h1>记一局</h1><p>选游戏、选朋友、记下结果。回忆可以慢慢补上。</p></div>
  <EmptyState title="记局功能正在准备中" description="当前预览用于确认页面结构。登录、小组和保存功能接入后，就可以在这里记录第一局。">
    <RouterLink to="/" class="button button-secondary">返回回顾</RouterLink>
  </EmptyState>
</template>
