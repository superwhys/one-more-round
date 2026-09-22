<script setup lang="ts">
import Icon from '@/components/common/AppIcon.vue'
import type { Page } from '@/types/journal'
defineProps<{ page: Page }>()
</script>

<template>
  <div class="d-overview journal-overview" aria-label="当前范围的对局统计">
    <div class="overview-stat">
      <Icon class="overview-icon" name="review" :size="19" />
      <strong>{{ page.total }}<small>局</small></strong
      ><span>一起度过的好时光</span>
    </div>
    <div class="overview-stat">
      <Icon class="overview-icon" name="game" :size="19" />
      <strong>{{ page.games }}<small>款</small></strong
      ><span>玩过的桌游</span>
    </div>
    <div class="overview-stat">
      <Icon class="overview-icon" name="group" :size="19" />
      <strong>{{ page.players }}<small>位</small></strong
      ><span>同桌的朋友</span>
    </div>
    <div class="d-overview-note">
      <Icon name="heart" /><span>最好的下一局，<br />是和你们一起。</span>
    </div>
  </div>
</template>

<style scoped>
.journal-overview {
  gap: 12px;
  padding: 0;
  border: 0;
  margin-bottom: 32px;
}
.journal-overview > .overview-stat {
  position: relative;
  min-width: 0;
  padding: 18px 20px;
  border: 1px solid var(--d-line);
  border-radius: 12px;
  background: var(--d-paper);
}
.overview-icon {
  position: absolute;
  top: 20px;
  right: 16px;
  color: #989e88;
}
.journal-overview .overview-stat > strong {
  padding-right: 20px;
  font-size: 34px;
  overflow-wrap: anywhere;
}
.journal-overview .overview-stat > span {
  color: #7e8571;
  font-size: 11px;
  white-space: normal;
}
.journal-overview .d-overview-note {
  padding: 12px;
  border: 0;
}
.journal-overview .d-overview-note > svg {
  color: #ae765d;
}
@media (max-width: 1150px) {
  .journal-overview {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .journal-overview .d-overview-note {
    display: none;
  }
}
@media (max-width: 760px) {
  .journal-overview {
    gap: 8px;
    margin-bottom: 28px;
  }
  .journal-overview > .overview-stat {
    padding: 14px 11px;
    border-radius: 10px;
  }
  .journal-overview .overview-stat > strong {
    padding: 0;
    font-size: 28px;
  }
  .journal-overview .overview-stat > span {
    font-size: 10px;
  }
  .overview-icon {
    position: static;
    margin-bottom: 12px;
    width: 17px;
    height: 17px;
  }
}
</style>
