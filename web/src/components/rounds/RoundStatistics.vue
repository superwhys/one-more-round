<script setup lang="ts">
import { computed, ref } from 'vue'
import { modeNames } from '@/utils/round'
import type { Snapshot, Stat, Mode } from '@/types/journal'
const props = defineProps<{ snapshot: Snapshot; items: Stat[]; playerId?: string }>()
const statMode = ref<Mode>('individual')
const stats = computed(() =>
  props.items.filter(stat => stat.mode === statMode.value && (!props.playerId || stat.player === props.playerId)),
)
const gameName = (id: string) => props.snapshot.games.find(game => game.id === id)?.name ?? '桌游'
const playerName = (id: string) => props.snapshot.players.find(player => player.id === id)?.name ?? '玩家'
</script>

<template>
  <section class="d-surface d-stat-section">
    <h2>每一款，都有自己的故事</h2>
    <div class="d-segment small">
      <button
        v-for="(label, mode) in modeNames"
        :key="mode"
        :class="{ active: statMode === mode }"
        :aria-pressed="statMode === mode"
        @click="statMode = mode"
      >
        {{ label }}
      </button>
    </div>
    <div v-for="s in stats" :key="`${s.game}:${s.player}:${s.mode}`" class="j-stat">
      <span class="j-stat-name">{{ gameName(s.game) }} · {{ playerName(s.player) }}</span>
      <span class="j-stat-result">
        <strong v-if="s.samples"
          >{{ Math.round((s.wins / s.samples) * 100) }}%<small>
            · {{ s.wins }} {{ s.mode === 'coop' ? '成功' : '胜' }} / {{ s.samples }} 局</small
          ></strong
        ><span v-else>暂无数据</span><small class="j-stat-played">{{ s.played }} 次参与</small>
      </span>
    </div>
    <p v-if="!stats.length" class="d-note">这个模式暂无数据。</p>
  </section>
</template>
