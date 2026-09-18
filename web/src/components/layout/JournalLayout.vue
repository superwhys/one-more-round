<script setup lang="ts">
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
defineProps<{ groupName: string; section: string; editor: boolean; back: boolean }>()
</script>

<template>
<aside class="d-sidebar"><RouterLink to="/" class="d-brand"><img src="/favicon.svg" alt="" /><span>又一局<small>ONE MORE ROUND</small></span></RouterLink><div class="d-group-switch"><span class="d-group-glyph">{{ Array.from(groupName ?? '局')[0] }}</span><span><strong>{{ groupName }}</strong><small>我们的桌游小天地</small></span></div><nav class="d-navigation" aria-label="主要导航"><RouterLink to="/" :class="{ active: section === 'review' }"><Icon name="review" />回顾</RouterLink><RouterLink to="/games" :class="{ active: section === 'games' }"><Icon name="game" />桌游</RouterLink><RouterLink to="/group" :class="{ active: section === 'group' }"><Icon name="group" />小组</RouterLink></nav><RouterLink to="/rounds/new" class="d-button d-sidebar-record"><Icon name="plus" />记一局</RouterLink><div class="d-sidebar-bottom"><div class="d-little-dice">⚄</div><p>输赢会忘记，<br />相聚值得留下。</p></div></aside>
  <div class="d-workspace"><header class="d-topbar"><RouterLink v-if="back" to="/" class="d-text-link">← 回到回顾</RouterLink><span v-else>OUR SHARED JOURNAL</span><span>邀请试用 · 小组私有</span></header><main id="main-content" class="d-main">
    <slot />
</main><footer class="d-footer">记下每一局的输赢与相聚。<span>又一局 · 私密的共同对局日记</span></footer></div>
  <RouterLink v-if="!editor" to="/rounds/new" class="d-button d-floating-record">＋ 记一局</RouterLink><nav v-if="!editor" class="d-mobile-nav" aria-label="手机导航"><RouterLink to="/"><Icon name="review" />回顾</RouterLink><RouterLink to="/games"><Icon name="game" />桌游</RouterLink><RouterLink to="/group"><Icon name="group" />小组</RouterLink></nav>
</template>
