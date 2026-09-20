<script setup lang="ts">
import { RouterView } from 'vue-router'
import favicon from '@/assets/favicon.svg'
</script>

<template>
  <RouterView v-slot="{ Component }">
    <component :is="Component" v-if="Component" />
    <Transition name="journal-opening">
      <main v-if="!Component" class="journal-opening" aria-label="又一局正在加载">
        <div class="opening-content">
          <div class="opening-journal" aria-hidden="true">
            <span class="opening-bookmark"></span>
            <img :src="favicon" width="48" height="48" alt="" />
            <span>桌边日记</span>
          </div>
          <p class="opening-eyebrow">ONE MORE ROUND</p>
          <h1>又一局<span>。</span></h1>
          <p class="opening-tagline">记下每一局的输赢与相聚。</p>
          <p class="opening-status" role="status">
            正在翻开对局日记
            <span class="opening-dots" aria-hidden="true"><i></i><i></i><i></i></span>
          </p>
        </div>
      </main>
    </Transition>
  </RouterView>
</template>

<style scoped>
.journal-opening {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: grid;
  place-items: center;
  overflow-y: auto;
  padding: 40px 24px;
  background: var(--d-bg, #f8f7f3);
  color: var(--d-ink, #35382f);
  text-align: center;
}

.opening-content {
  margin: auto;
}

.opening-journal {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  width: 116px;
  height: 142px;
  margin: 0 auto 32px;
  border: 1px solid #dadbcc;
  border-radius: 5px 12px 12px 5px;
  background: #eeeee3;
  box-shadow:
    3px 3px 0 -1px #fffefa,
    4px 4px 0 0 #dadbcc,
    0 16px 28px -16px #35382f38;
  transform: rotate(-7deg);
}

.opening-journal::before {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 10px;
  border-left: 1px solid #d5d8c8;
  content: '';
}

.opening-journal > img {
  display: block;
}

.opening-journal > span:last-child {
  color: var(--d-green, #617461);
  font-size: 10px;
  letter-spacing: 3px;
}

.opening-bookmark {
  position: absolute;
  top: -1px;
  right: 16px;
  width: 12px;
  height: 25px;
  background: var(--d-accent, #bc5a3b);
  clip-path: polygon(0 0, 100% 0, 100% 100%, 50% 78%, 0 100%);
}

.opening-eyebrow {
  margin: 0 0 10px;
  color: #79806e;
  font-size: 9px;
  font-weight: 500;
  letter-spacing: 3px;
}

.opening-content h1 {
  margin: 0;
  font-family: 'Songti SC', 'Noto Serif CJK SC', 'STSong', serif;
  font-size: 38px;
  font-weight: 700;
  line-height: 1.4;
  letter-spacing: 3px;
}

.opening-content h1 > span {
  color: var(--d-accent, #bc5a3b);
}

.opening-tagline {
  margin: 12px 0 0;
  color: #747969;
  font-size: 13px;
  line-height: 1.8;
}

.opening-status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin: 36px 0 0;
  color: #747969;
  font-size: 11px;
  line-height: 1.8;
}

.opening-dots {
  display: flex;
  gap: 4px;
}

.opening-dots > i {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--d-accent, #bc5a3b);
  opacity: 0.25;
  /* A slow loop indicates an ongoing wait without flashing. */
  animation: opening-pulse 1200ms linear infinite;
}

.opening-dots > i:nth-child(2) {
  animation-delay: 200ms;
}

.opening-dots > i:nth-child(3) {
  animation-delay: 400ms;
}

@keyframes opening-pulse {
  50% {
    opacity: 1;
  }
}

.journal-opening-leave-active {
  pointer-events: none;
  transition: opacity 200ms cubic-bezier(0.23, 1, 0.32, 1);
}

.journal-opening-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .opening-dots > i {
    animation: none;
    opacity: 0.7;
  }
}
</style>
