<script setup lang="ts">
import { RouterLink } from 'vue-router'
import EmptyState from '@/components/common/EmptyState.vue'
</script>

<template>
  <div class="page-heading"><p class="eyebrow">我们的对局日记</p><h1>好玩的，值得记下来。</h1><p>一场游戏、几个朋友，还有那些值得回味的瞬间。</p></div>
  <div class="section-heading"><h2>一起玩过的日子</h2><span>从第一局开始</span></div>
  <EmptyState title="为下一次相聚，留个位置" description="这里将按时间收集你们的对局、照片与回忆。当前为界面预览，记录功能尚未开放。" symbol="◷">
    <RouterLink to="/group" class="button button-secondary">看看小组</RouterLink>
  </EmptyState>
</template>
