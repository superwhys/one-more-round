<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { games, gameRounds } from '@/stores/designPreview'
import Icon from '@/components/common/AppIcon.vue'
import Modal from '@/components/common/AppModal.vue'
import GameCover from '@/components/design/GameCover.vue'
const gameSearch = ref('')
const filteredGames = computed(() => games.filter((g) => `${g.name} ${g.english}`.toLowerCase().includes(gameSearch.value.toLowerCase())))
const manualName = ref('')
const router = useRouter()
const modal = ref('')
function addGame() {
  const name = manualName.value.trim()
  if (!name) return
  const existing = games.find((g) => g.name === name)
  if (existing) { modal.value = ''; void router.push(`/design/games/${existing.id}`); return }
  const id = crypto.randomUUID()
  games.push({ id, name, english: 'OUR GAME', theme: 'sun', motif: '✳', caption: '新的游戏，新的故事。' })
  modal.value = ''; manualName.value = ''; void router.push(`/design/games/${id}`)
}
</script>

<template>
<header class="d-page-heading"><div><p class="d-eyebrow">FAVORITES ON OUR TABLE</p><h1>我们的桌游架<span class="d-title-dot">。</span></h1><p>每一盒打开的，都是一起玩的理由。</p></div><button class="d-button" @click="modal = 'add-game'"><Icon name="plus" :size="17" />添加桌游</button></header>
          <div class="d-game-toolbar"><label class="d-search"><Icon name="search" :size="18" /><input v-model="gameSearch" placeholder="找找桌游名称、中文别名…" aria-label="搜索游戏架" /></label><span>{{ filteredGames.length }} 款本组桌游</span></div>
          <div class="d-game-grid"><RouterLink v-for="game in filteredGames" :key="game.id" :to="`/design/games/${game.id}`" class="d-game-tile"><GameCover :game="game" /><div><h2>{{ game.name }}</h2><p>{{ game.caption }}</p><footer><span>一起玩过 <strong>{{ gameRounds(game.id).length }}</strong> 局</span><Icon name="arrow" :size="17" /></footer></div></RouterLink></div><div v-if="!filteredGames.length" class="d-empty"><h3>还没在这张桌子上见过它</h3><p>换个名称试试，或者手动添加。</p><button class="d-button secondary" @click="manualName = gameSearch; modal = 'add-game'">手动添加</button></div>
<Modal v-if="modal === 'add-game'" title="把它放上游戏架。" @close="modal = ''"><p class="d-modal-copy">只需要一个名称，就能开始记录。</p><label class="d-field">桌游名称<input v-model="manualName" placeholder="比如：我们常玩的那款桌游" @keyup.enter="addGame" /></label><button class="d-button full" @click="addGame">添加到本组桌游</button><p class="d-note">本稿使用原创示意封面。外部桌游资料检索尚未接入。</p></Modal>
</template>
