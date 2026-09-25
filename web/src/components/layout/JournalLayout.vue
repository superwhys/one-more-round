<script setup lang="ts">
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import BggAttribution from '@/components/games/BggAttribution.vue'
import favicon from '@/assets/favicon.svg'
defineProps<{ groupName: string; section: string; editor: boolean }>()
</script>

<template>
  <aside class="d-sidebar">
    <RouterLink to="/" class="d-brand"
      ><img :src="favicon" alt="" /><span>又一局<small>ONE MORE ROUND</small></span></RouterLink
    >
    <div class="d-group-switch">
      <span class="d-group-glyph">{{ Array.from(groupName ?? '局')[0] }}</span
      ><span
        ><strong>{{ groupName }}</strong
        ><small>我们的桌游小天地</small></span
      >
    </div>
    <p class="navigation-label">我们的日记</p>
    <nav class="d-navigation" aria-label="主要导航">
      <RouterLink
        to="/"
        :class="{ active: section === 'review' }"
        :aria-current="section === 'review' ? 'page' : undefined"
        ><Icon name="review" />回顾<small aria-hidden="true">一起玩过的日子</small></RouterLink
      ><RouterLink
        to="/games"
        :class="{ active: section === 'games' }"
        :aria-current="section === 'games' ? 'page' : undefined"
        ><Icon name="game" />桌游<small aria-hidden="true">收藏每一种乐趣</small></RouterLink
      ><RouterLink
        to="/group"
        :class="{ active: section === 'group' }"
        :aria-current="section === 'group' ? 'page' : undefined"
        ><Icon name="group" />小组<small aria-hidden="true">这张桌上的朋友</small></RouterLink
      >
    </nav>
    <RouterLink to="/rounds/new" class="d-button d-sidebar-record"><Icon name="plus" />记一局</RouterLink>
    <div class="d-sidebar-bottom">
      <div class="d-little-dice">⚄</div>
      <p>输赢会忘记，<br />相聚值得留下。</p>
    </div>
  </aside>
  <div class="d-workspace">
    <main id="main-content" class="d-main">
      <slot />
    </main>
    <footer class="d-footer">
      <span>记下每一局的输赢与相聚。</span>
      <span>又一局 · 私密的共同对局日记</span>
      <BggAttribution class="d-footer-bgg" />
    </footer>
  </div>
  <RouterLink v-if="!editor" to="/rounds/new" class="d-button d-floating-record">＋ 记一局</RouterLink>
  <nav v-if="!editor" class="d-mobile-nav" aria-label="手机导航">
    <RouterLink
      to="/"
      :class="{ active: section === 'review' }"
      :aria-current="section === 'review' ? 'page' : undefined"
      ><Icon name="review" />回顾</RouterLink
    ><RouterLink
      to="/games"
      :class="{ active: section === 'games' }"
      :aria-current="section === 'games' ? 'page' : undefined"
      ><Icon name="game" />桌游</RouterLink
    ><RouterLink
      to="/group"
      :class="{ active: section === 'group' }"
      :aria-current="section === 'group' ? 'page' : undefined"
      ><Icon name="group" />小组</RouterLink
    >
  </nav>
</template>

<style scoped>
.d-footer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
}
.d-footer > span:nth-child(2) {
  grid-column: 3;
  grid-row: 1;
  justify-self: end;
  text-align: right;
}
.d-footer-bgg {
  grid-column: 2;
  grid-row: 1;
  justify-self: center;
}
.d-sidebar {
  background: #efeee5;
}
.d-group-switch {
  border-radius: 12px;
  margin-bottom: 26px;
}
.navigation-label {
  padding: 0 15px 12px;
  color: #929581;
  font-size: 9px;
  letter-spacing: 1.8px;
}
.d-navigation a {
  position: relative;
  display: grid;
  grid-template-columns: 21px 1fr;
  gap: 1px 13px;
  padding-block: 12px;
  border-radius: 10px;
}
.d-navigation a > svg {
  grid-row: span 2;
}
.d-navigation small {
  grid-column: 2;
  font-size: 9px;
  font-weight: 400;
  color: #979984;
}
.d-navigation a::after {
  content: '';
  position: absolute;
  left: 0;
  top: 19px;
  bottom: 19px;
  width: 3px;
  border-radius: 2px;
  background: var(--d-accent);
  opacity: 0;
  transition: opacity 160ms var(--ease-out);
}
.d-navigation a.active::after {
  opacity: 1;
}
.d-navigation a.active small {
  color: #a67a60;
}
.d-navigation a:focus-visible::after {
  transition: none;
}
.d-sidebar-record {
  border-radius: 10px;
}
.d-mobile-nav > a {
  position: relative;
  gap: 4px;
  border-radius: 10px;
}
.d-mobile-nav > a.active {
  background: #f3ebe0;
}
.d-mobile-nav > a::before {
  content: '';
  position: absolute;
  top: 0;
  width: 18px;
  height: 2px;
  border-radius: 2px;
  background: var(--d-accent);
  opacity: 0;
  transition: opacity 160ms var(--ease-out);
}
.d-mobile-nav > a.active::before {
  opacity: 1;
}
.d-mobile-nav > a:focus-visible::before {
  transition: none;
}
@media (max-width: 760px) {
  .d-footer {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
  }
  .d-footer > span:nth-child(2) {
    text-align: center;
  }
  .d-footer-bgg {
    margin-top: 10px;
  }
  .d-mobile-nav {
    gap: 10px;
  }
}
</style>
