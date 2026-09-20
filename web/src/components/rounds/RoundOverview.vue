<script setup lang="ts">
import Icon from '@/components/common/AppIcon.vue'
import type { Page } from '@/types/journal'
defineProps<{ page: Page }>()
</script>

<template>
  <div class="d-overview">
    <div>
      <strong>{{ page.total }}<small>局</small></strong
      ><span>一起度过的好时光</span>
    </div>
    <div>
      <strong>{{ page.games }}<small>款</small></strong
      ><span>玩过的桌游</span>
    </div>
    <div>
      <strong>{{ page.players }}<small>位</small></strong
      ><span>同桌的朋友</span>
    </div>
    <div class="d-overview-note">
      <Icon name="heart" /><span>最好的下一局，<br />是和你们一起。</span>
    </div>
  </div>
</template>
