<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import Modal from '@/components/common/AppModal.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'

import { addGame, searchBGG as search } from '@/api/game'
import { useRoundHistory } from '@/composables/useRoundHistory'
const { groupId, snapshot, refresh } = useGroupContext()
const { page, loading, error: loadError, load } = useRoundHistory()
const { busy, error, notice, run } = useGroupOperation()
const gameSearch = ref('')
const name = ref('')
const addingOpen = ref(false)
const filteredGames = computed(
  () =>
    snapshot.value?.games.filter(game =>
      `${game.name} ${game.original}`.toLowerCase().includes(gameSearch.value.toLowerCase()),
    ) ?? [],
)
const gameCount = (id: string) => {
  const activity = page.value.activity[id]
  return activity ? `${activity.count} 局 · 最近 ${activity.last_date}` : '还没记过局'
}
function openAddForm(initialName = '') {
  addingOpen.value = true
  name.value = initialName
}
function add() {
  return run(async () => {
    await addGame(groupId, name.value)
    name.value = ''
    addingOpen.value = false
    notice.value = '已经添加，可以开始记局了'
    try {
      await refresh()
    } catch {
      notice.value = '桌游已添加，刷新失败，请重新加载'
    }
  })
}
function searchBGG() {
  return run(async () => {
    await search(groupId, gameSearch.value)
  })
}
function retry() {
  return run(async () => {
    await refresh()
    await load()
  })
}
</script>

<template>
  <template v-if="snapshot"
    ><RequestStatus
      :loading="loading"
      :error="loadError || error"
      :notice="notice"
      @retry="retry"
      @dismiss="notice = ''"
    />
    <header class="d-page-heading">
      <div>
        <p class="d-eyebrow">FAVORITES ON OUR TABLE</p>
        <h1>我们的桌游架<span class="d-title-dot">。</span></h1>
        <p>每一盒打开的，都是一起玩的理由。</p>
      </div>
      <button class="d-button" @click="openAddForm()">＋ 添加桌游</button>
    </header>
    <div class="d-game-toolbar">
      <label class="d-search"
        ><Icon name="search" /><input
          v-model="gameSearch"
          type="search"
          placeholder="名称 / 中文别名"
          aria-label="搜索本组桌游" /></label
      ><button class="d-text-link" :disabled="busy" @click="searchBGG">搜索更多桌游</button>
    </div>
    <div class="d-game-grid">
      <RouterLink v-for="g in filteredGames" :key="g.id" :to="`/games/${g.id}`" class="d-game-tile"
        ><div class="d-cover sun">
          <span class="d-cover-orbit"></span><span class="d-cover-motif">✳</span><strong>OUR GAME</strong>
        </div>
        <div>
          <h2>{{ g.name }}</h2>
          <p>{{ g.original || '新的游戏，新的故事。' }}</p>
          <footer>
            <span>{{ gameCount(g.id) }}</span
            ><Icon name="arrow" />
          </footer></div
      ></RouterLink>
    </div>
    <div v-if="!filteredGames.length" class="d-empty">
      <h3>游戏架还留着位置。</h3>
      <p>先手动添加一款常玩的桌游吧。</p>
      <button class="d-button secondary" @click="openAddForm(gameSearch)">手动添加</button>
    </div>
    <Modal v-if="addingOpen" title="把它放上游戏架。" @close="addingOpen = false"
      ><form @submit.prevent="add">
        <label class="d-field">桌游名称<input v-model="name" required maxlength="255" autofocus /></label>
        <p v-if="error" class="j-error" role="alert">{{ error }}</p>
        <button class="d-button full" :disabled="busy">保存</button>
      </form></Modal
    >
  </template>
</template>
