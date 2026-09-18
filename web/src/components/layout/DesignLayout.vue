<script setup lang="ts">
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import Avatar from '@/components/design/PlayerAvatar.vue'
defineProps<{ isEditor: boolean; section: string; back: boolean; title: string; groupName: string; roundCount: number }>()
defineEmits<{ group: []; guide: []; welcome: []; home: [] }>()
</script>

<template>
<a class="skip-link" href="#design-main">跳到主要内容</a>
    <div class="d-review-strip"><span><span class="d-live-dot"></span>产品稿 V0.1 <i>·</i> 示例数据，仅供评审</span><div><button @click="$emit('guide')">体验指南</button><button @click="$emit('welcome')">首次使用</button></div></div>
    <aside class="d-sidebar">
      <RouterLink to="/design" class="d-brand"><img src="/favicon.svg" alt="" /><span>又一局<small>ONE MORE ROUND</small></span></RouterLink>
      <button class="d-group-switch" @click="$emit('group')"><span class="d-group-glyph">五</span><span><strong>{{ groupName }}</strong><small>我们的桌游小天地</small></span><Icon name="down" :size="15" /></button>
      <nav class="d-navigation" aria-label="产品稿导航">
        <RouterLink to="/design" :class="{ active: section === 'review' }"><Icon name="review" />回顾<span>{{ String(roundCount).padStart(2, '0') }}</span></RouterLink>
        <RouterLink to="/design/games" :class="{ active: section === 'games' }"><Icon name="game" />桌游</RouterLink>
        <RouterLink to="/design/group" :class="{ active: section === 'group' }"><Icon name="group" />小组</RouterLink>
      </nav>
      <RouterLink to="/design/new" class="d-button d-sidebar-record"><Icon name="plus" :size="19" />记一局</RouterLink>
      <div class="d-sidebar-bottom"><div class="d-little-dice"><span>•</span><span>••</span></div><p>输赢会忘记，<br />相聚值得留下。</p><div class="d-current-user"><Avatar id="lin" small /><span>阿林<small>今天也想再来一局</small></span><span class="d-owner-tag">组主</span></div></div>
    </aside>
    <div class="d-workspace">
      <header class="d-topbar"><div><button v-if="back" class="d-icon-button" aria-label="返回回顾" @click="$emit('home')"><Icon name="back" /></button><span v-else class="d-breadcrumb">我们的桌游日记</span><span v-if="isEditor">{{ title }}</span><span v-else-if="back">{{ title }}</span></div><button class="d-mobile-group" @click="$emit('group')">{{ groupName }}<Icon name="down" :size="14" /></button><span class="d-private"><Icon name="group" :size="15" />只和小组里的朋友分享</span><Avatar id="lin" small /></header>
      <main id="design-main" class="d-main" :class="{ 'editor-main': isEditor }" tabindex="-1">
<slot /></main>
      <footer class="d-footer">记下每一局的输赢与相聚。<span>又一局 · 私密的共同对局日记</span></footer>
    </div>
    <RouterLink v-if="!isEditor" to="/design/new" class="d-button d-floating-record"><Icon name="plus" :size="19" />记一局</RouterLink>
    <nav v-if="!isEditor" class="d-mobile-nav" aria-label="手机产品稿导航"><RouterLink to="/design" :class="{ active: section === 'review' }"><Icon name="review" />回顾</RouterLink><RouterLink to="/design/games" :class="{ active: section === 'games' }"><Icon name="game" />桌游</RouterLink><RouterLink to="/design/group" :class="{ active: section === 'group' }"><Icon name="group" />小组</RouterLink></nav>
</template>
