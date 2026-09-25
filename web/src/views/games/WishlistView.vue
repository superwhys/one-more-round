<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { addWishlistGame, getWishlist, removeWishlistGame } from '@/api/wishlist'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'
import { message } from '@/utils/error'

const { groupId, snapshot, handleAccessError } = useGroupContext()
const { busy, error, notice, run } = useGroupOperation()
const loading = ref(true)
const loadError = ref('')
const wishedIDs = ref<string[]>([])
const selectedGame = ref('')
const wishedGames = computed(() => snapshot.value?.games.filter(game => wishedIDs.value.includes(game.id)) ?? [])
const availableGames = computed(() => snapshot.value?.games.filter(game => !wishedIDs.value.includes(game.id)) ?? [])

// load fetches the group's shared wishlist and keeps failures separate from an empty list.
async function load() {
  loading.value = true
  loadError.value = ''
  try {
    wishedIDs.value = (await getWishlist(groupId)).game_ids
  } catch (cause) {
    loadError.value = message(cause)
    await handleAccessError(cause)
  } finally {
    loading.value = false
  }
}

// add marks the selected shelf game as wanted after the server confirms it.
function add() {
  if (!selectedGame.value) return
  const gameId = selectedGame.value
  return run(async () => {
    await addWishlistGame(groupId, gameId)
    wishedIDs.value = [...wishedIDs.value, gameId]
    selectedGame.value = ''
    notice.value = '已加入想玩清单'
  })
}

// remove clears a game from the shared list after the server confirms it.
function remove(gameId: string) {
  return run(async () => {
    await removeWishlistGame(groupId, gameId)
    wishedIDs.value = wishedIDs.value.filter(id => id !== gameId)
    notice.value = '已移出想玩清单'
  })
}

onMounted(load)
</script>

<template>
  <template v-if="snapshot">
    <header class="d-page-heading">
      <div>
        <p class="d-eyebrow">NEXT TIME AT OUR TABLE</p>
        <h1>下次想玩<span class="d-title-dot">。</span></h1>
        <p>把想一起打开的游戏记在这里，下次聚会就有答案。</p>
      </div>
      <RouterLink to="/games" class="d-button secondary"><Icon name="game" :size="17" /> 返回桌游架</RouterLink>
    </header>

    <RequestStatus
      :loading="loading"
      :error="loadError || error"
      :notice="notice"
      @retry="load"
      @dismiss="notice = ''"
    />

    <section v-if="!loading && !loadError" class="wishlist-section">
      <form v-if="availableGames.length" class="d-surface wishlist-add" @submit.prevent="add">
        <label class="d-field">
          从本组桌游架挑一款
          <select v-model="selectedGame" required :disabled="busy">
            <option value="">选择桌游</option>
            <option v-for="game in availableGames" :key="game.id" :value="game.id">{{ game.name }}</option>
          </select>
        </label>
        <button class="d-button" :disabled="busy || !selectedGame">加入想玩清单</button>
      </form>
      <p v-else-if="!snapshot.games.length" class="d-note">
        本组还没有桌游。<RouterLink to="/games">先添加一款</RouterLink>，就能把它放进想玩清单。
      </p>

      <div v-if="wishedGames.length" class="wishlist-grid">
        <article v-for="game in wishedGames" :key="game.id" class="d-surface wishlist-card">
          <img v-if="game.cover" :src="game.cover" :alt="game.name" class="wishlist-cover" />
          <div v-else class="wishlist-cover wishlist-cover-empty" aria-hidden="true">
            <Icon name="game" :size="30" />
          </div>
          <div class="wishlist-card-body">
            <RouterLink :to="`/games/${game.id}`" class="wishlist-title">{{ game.name }}</RouterLink>
            <p v-if="game.original && game.original !== game.name">{{ game.original }}</p>
            <div class="wishlist-actions">
              <RouterLink :to="`/rounds/new?game=${game.id}`" class="d-button">记一局</RouterLink>
              <button type="button" class="d-button secondary" :disabled="busy" @click="remove(game.id)">
                移出清单
              </button>
            </div>
          </div>
        </article>
      </div>
      <div v-else class="d-empty">
        <h2>下次玩什么？</h2>
        <p>从桌游架挑一款，大家都能看到这份想玩清单。</p>
      </div>
    </section>
  </template>
</template>

<style scoped>
.wishlist-section {
  display: grid;
  gap: 22px;
}
.wishlist-add {
  display: flex;
  align-items: end;
  gap: 14px;
  padding: 20px;
}
.wishlist-add .d-field {
  flex: 1;
  margin: 0;
}
.wishlist-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 380px), 1fr));
}
.wishlist-card {
  display: flex;
  gap: 14px;
  min-width: 0;
  padding: 16px;
}
.wishlist-cover {
  flex: 0 0 78px;
  width: 78px;
  height: 104px;
  object-fit: cover;
  border-radius: 7px;
}
.wishlist-cover-empty {
  display: grid;
  place-items: center;
  color: #837a69;
  background: #e8dfd0;
}
.wishlist-card-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 6px;
}
.wishlist-title {
  color: var(--d-ink);
  font-weight: 700;
  overflow-wrap: anywhere;
}
.wishlist-card-body p {
  color: #777b69;
  font-size: 12px;
}
.wishlist-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: auto;
  padding-top: 12px;
}
@media (max-width: 540px) {
  .wishlist-add {
    align-items: stretch;
    flex-direction: column;
  }
  .wishlist-add .d-field,
  .wishlist-add .d-button {
    width: 100%;
  }
}
@media (max-width: 360px) {
  .wishlist-actions .d-button {
    padding-inline: 12px;
  }
}
</style>
