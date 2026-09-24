<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import Modal from '@/components/common/AppModal.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'

import { addGame, importGame, searchBGG as search, syncCover, type ExternalSearch } from '@/api/game'
import type { Game } from '@/types/journal'
import { useRoundHistory } from '@/composables/useRoundHistory'
const { groupId, snapshot, refresh } = useGroupContext()
const { page, loading, error: loadError, load } = useRoundHistory()
const { busy, error, notice, run } = useGroupOperation()
const gameSearch = ref('')
const name = ref('')
const addingOpen = ref(false)
const importingID = ref<number | null>(null)
const importingOriginal = ref('')
const externalOpen = ref(false)
const external = ref<ExternalSearch | null>(null)
const linkingGame = ref<Game | null>(null)
const searchTerm = computed(() => gameSearch.value.trim())
const filteredGames = computed(
  () =>
    snapshot.value?.games.filter(game =>
      `${game.name} ${game.original}`.toLowerCase().includes(searchTerm.value.toLowerCase()),
    ) ?? [],
)
function coverTone(id: string) {
  let hash = 0
  for (const character of id) hash = (hash * 31 + character.charCodeAt(0)) >>> 0
  return ['sage', 'clay', 'slate', 'wheat'][hash % 4]
}
const gameCount = (id: string) => {
  if (loading.value) return '正在整理记录…'
  if (loadError.value) return '记录暂未加载'
  const activity = page.value.activity[id]
  return activity ? `${activity.count} 局相聚` : '等待第一局'
}
function openAddForm(initialName = '') {
  importingID.value = null
  importingOriginal.value = ''
  addingOpen.value = true
  name.value = initialName
}
function chooseExternal(id: number, original: string) {
  const target = linkingGame.value
  if (target) {
    return run(async () => {
      await syncCover(groupId, target.id, id)
      linkingGame.value = null
      externalOpen.value = false
      notice.value = '封面已经同步'
      try {
        await refresh()
      } catch {
        notice.value = '封面已同步，刷新失败，请重新加载'
      }
    })
  }
  importingID.value = id
  importingOriginal.value = original
  name.value = original
  externalOpen.value = false
  addingOpen.value = true
}
function syncGameCover(game: Game) {
  if (game.bgg_id) {
    const bggID = game.bgg_id
    return run(async () => {
      await syncCover(groupId, game.id, bggID)
      notice.value = '封面已经同步'
      try {
        await refresh()
      } catch {
        notice.value = '封面已同步，刷新失败，请重新加载'
      }
    })
  }
  return run(async () => {
    linkingGame.value = game
    external.value = await search(groupId, game.name)
    externalOpen.value = true
  })
}
function add() {
  return run(async () => {
    const alias = name.value.trim()
    if (importingID.value) {
      await importGame(groupId, importingID.value, alias)
    } else {
      await addGame(groupId, alias)
    }
    name.value = ''
    importingID.value = null
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
    if (!searchTerm.value) {
      error.value = '先输入要找的桌游名称'
      return
    }
    external.value = await search(groupId, searchTerm.value)
    externalOpen.value = true
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
  <template v-if="snapshot">
    <RequestStatus
      :loading="loading"
      :error="loadError || error"
      :notice="notice"
      @retry="retry"
      @dismiss="notice = ''"
    />
    <header class="d-page-heading shelf-heading">
      <div>
        <p class="d-eyebrow">FAVORITES ON OUR TABLE</p>
        <h1>我们的桌游架<span class="d-title-dot">。</span></h1>
        <p>每一盒打开的，都是一起玩的理由。</p>
      </div>
      <button class="d-button" @click="openAddForm()"><Icon name="plus" :size="16" /> 添加桌游</button>
    </header>

    <section class="shelf-toolbar" aria-label="桌游架与搜索">
      <div class="shelf-count">
        <span class="shelf-count-icon"><Icon name="game" :size="22" /></span>
        <p>
          <strong>{{ snapshot.games.length }}</strong
          ><span>款桌游，在我们的架上</span>
        </p>
      </div>
      <label class="d-search shelf-search">
        <Icon name="search" :size="18" />
        <input v-model="gameSearch" type="search" placeholder="找一款桌游，或搜中文别名" aria-label="搜索本组桌游" />
      </label>
      <button class="d-text-link shelf-external-search" :disabled="busy" @click="searchBGG">
        搜索更多桌游 <Icon name="arrow" :size="15" />
      </button>
    </section>
    <div class="shelf-caption">
      <p aria-live="polite">
        {{ searchTerm ? `找到 ${filteredGames.length} 款桌游` : '熟悉的老朋友，下一局的新故事。' }}
      </p>
      <span v-if="!searchTerm && snapshot.games.length" aria-hidden="true">OUR COLLECTION</span>
    </div>

    <div v-if="filteredGames.length" class="shelf-grid">
      <RouterLink v-for="game in filteredGames" :key="game.id" :to="`/games/${game.id}`" class="shelf-card">
        <img v-if="game.cover" class="shelf-photo" :src="game.cover" :alt="game.name" />
        <div v-else class="shelf-cover" :class="coverTone(game.id)" aria-hidden="true">
          <div class="shelf-cover-frame"></div>
          <span class="shelf-cover-label">ONE MORE ROUND</span>
          <span class="shelf-cover-letter">{{ Array.from(game.name.trim())[0]?.toUpperCase() }}</span>
          <span class="shelf-cover-mark"><Icon name="game" :size="20" /></span>
          <span class="shelf-cover-note">好游戏，一起玩。</span>
        </div>
        <button v-if="!game.cover" class="shelf-sync" type="button" @click.prevent.stop="syncGameCover(game)">
          同步封面
        </button>
        <div class="shelf-card-body">
          <h2 :title="game.name">{{ game.name }}</h2>
          <p v-if="game.original && game.original !== game.name" class="shelf-original" :title="game.original">
            {{ game.original }}
          </p>
          <footer>
            <div class="shelf-activity">
              <span>{{ gameCount(game.id) }}</span>
              <small v-if="!loading && !loadError && page.activity[game.id]"
                >最近 {{ page.activity[game.id]?.last_date }}</small
              >
            </div>
            <span class="shelf-card-arrow"><Icon name="arrow" :size="17" /></span>
          </footer>
        </div>
      </RouterLink>
    </div>
    <div v-else class="d-empty shelf-empty">
      <span class="shelf-empty-icon"><Icon :name="searchTerm ? 'search' : 'game'" :size="30" /></span>
      <h3>{{ searchTerm ? '这款桌游，还没在架上。' : '游戏架还留着位置。' }}</h3>
      <p>
        {{
          searchTerm
            ? `没有找到「${searchTerm}」，换个名字找找，或把它加进来。`
            : '把大家常玩的那一盒放上来，一起留下第一局。'
        }}
      </p>
      <div class="shelf-empty-actions">
        <button v-if="searchTerm" class="d-button secondary" @click="gameSearch = ''">查看全部桌游</button>
        <button class="d-button" @click="openAddForm(searchTerm)">＋ 手动添加</button>
      </div>
    </div>
    <Modal
      v-if="externalOpen && external"
      :title="linkingGame ? '选择一张封面。' : '搜索更多桌游。'"
      wide
      @close="
        () => {
          externalOpen = false
          linkingGame = null
        }
      "
    >
      <p v-if="external.items.length === 0" class="shelf-external-empty">没有找到这款。可以换原文名称，或手动添加。</p>
      <ul v-else class="shelf-external-list">
        <li v-for="item in external.items" :key="item.bgg_id">
          <img v-if="item.thumbnail" class="shelf-external-cover" :src="item.thumbnail" :alt="item.name" />
          <span v-else class="shelf-external-cover shelf-external-cover-empty" aria-hidden="true"></span>
          <div>
            <strong>{{ item.name }}</strong>
            <span v-if="item.year">{{ item.year }}</span>
          </div>
          <button class="d-button secondary" type="button" @click="chooseExternal(item.bgg_id, item.name)">
            {{ linkingGame ? '使用这张封面' : '放入游戏架' }}
          </button>
        </li>
      </ul>
      <p class="shelf-external-credit">
        资料来自
        <a :href="external.source_url" target="_blank" rel="noreferrer">{{ external.source }}</a>
      </p>
    </Modal>
    <Modal v-if="addingOpen" title="把它放上游戏架。" @close="addingOpen = false">
      <form @submit.prevent="add">
        <label class="d-field">
          {{ importingID ? '本组名称' : '桌游名称' }}
          <input v-model="name" :required="!importingID" maxlength="255" autofocus :placeholder="importingOriginal" />
        </label>
        <p v-if="importingOriginal" class="shelf-external-credit">
          架上会显示这个名称。BGG 的正式名称会另外保存，留空则改用正式名称。
        </p>
        <p v-if="error" class="j-error" role="alert">{{ error }}</p>
        <button class="d-button full" :disabled="busy">保存</button>
      </form>
    </Modal>
  </template>
</template>

<style scoped>
.shelf-heading {
  margin-bottom: 28px;
}
.shelf-toolbar {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 18px 20px;
  border: 1px solid var(--d-line);
  border-radius: 14px;
  background: var(--d-paper);
}
.shelf-count {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 0;
}
.shelf-count-icon,
.shelf-empty-icon {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  color: var(--d-green);
  background: #edf0e8;
}
.shelf-count p {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  column-gap: 8px;
  row-gap: 2px;
}
.shelf-count strong {
  font-family: Georgia, 'Songti SC', serif;
  font-size: 28px;
  line-height: 1;
  font-weight: 400;
  font-variant-numeric: tabular-nums;
}
.shelf-count p > span {
  font-size: 11px;
  color: var(--d-muted);
}
.shelf-search {
  flex: 1;
  max-width: 320px;
  background: var(--d-bg);
}
.shelf-external-search {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 44px;
  white-space: nowrap;
}
.shelf-caption {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin: 25px 0 16px;
  color: var(--d-muted);
  font-size: 11px;
}
.shelf-caption > span {
  font-size: 9px;
  letter-spacing: 1.8px;
  white-space: nowrap;
}
.shelf-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 24px;
}
.shelf-photo {
  width: 100%;
  aspect-ratio: 1.3;
  object-fit: cover;
  border-radius: 7px;
  background: #edf0e8;
}
.shelf-sync {
  align-self: flex-start;
  margin: 8px 7px 0;
  padding: 0;
  border: 0;
  background: none;
  color: var(--d-green);
  font-size: 12px;
  cursor: pointer;
}
.shelf-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--d-line);
  border-radius: 14px;
  background: var(--d-paper);
  box-shadow: 0 3px 0 #ebe9e1;
  transition: transform 160ms var(--ease-out);
}
.shelf-card:focus-visible {
  outline: 2px solid var(--d-accent);
  outline-offset: 4px;
}
.shelf-cover {
  --cover-paper: #dfe5d6;
  --cover-ink: #637557;
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  aspect-ratio: 1.3;
  overflow: hidden;
  border-radius: 7px;
  isolation: isolate;
  color: var(--cover-ink);
  background: var(--cover-paper);
  box-shadow:
    inset 7px 0 0 rgb(255 255 255 / 23%),
    inset 8px 0 0 rgb(0 0 0 / 7%);
}
.shelf-cover.clay {
  --cover-paper: #eddbce;
  --cover-ink: #a1694e;
}
.shelf-cover.slate {
  --cover-paper: #dce4e6;
  --cover-ink: #617985;
}
.shelf-cover.wheat {
  --cover-paper: #eae3cb;
  --cover-ink: #97814c;
}
.shelf-cover::before,
.shelf-cover::after {
  content: '';
  position: absolute;
  z-index: -1;
  width: 62%;
  aspect-ratio: 1;
  border: 1px solid currentColor;
  border-radius: 50%;
  opacity: 0.15;
}
.shelf-cover::before {
  right: -23%;
  top: -22%;
}
.shelf-cover::after {
  left: -32%;
  bottom: -36%;
  width: 94%;
}
.shelf-cover-frame {
  position: absolute;
  inset: 13px 12px 13px 19px;
  border: 1px solid currentColor;
  border-radius: 2px;
  opacity: 0.23;
}
.shelf-cover-label {
  position: absolute;
  top: 24px;
  font-size: 8px;
  font-weight: 600;
  letter-spacing: 2px;
}
.shelf-cover-letter {
  font-family: 'Songti SC', 'Noto Serif CJK SC', 'STSong', serif;
  font-size: clamp(48px, 5.8vw, 76px);
  font-weight: 700;
  line-height: 1;
}
.shelf-cover-mark {
  position: absolute;
  right: 22px;
  bottom: 24px;
  opacity: 0.6;
  transform: rotate(12deg);
}
.shelf-cover-note {
  position: absolute;
  bottom: 25px;
  left: 29px;
  font-size: 9px;
  letter-spacing: 1px;
}
.shelf-card-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  padding: 17px 7px 7px;
  min-width: 0;
}
.shelf-card h2 {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  overflow-wrap: anywhere;
  font-size: 16px;
  line-height: 1.55;
}
.shelf-original {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: var(--d-muted);
  font-size: 10px;
  margin-top: 5px;
}
.shelf-card footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 15px;
  margin-top: auto;
}
.shelf-activity {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
  font-size: 11px;
  color: var(--d-green);
}
.shelf-activity small {
  font-size: 9px;
  color: var(--d-muted);
}
.shelf-card-arrow {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: #f0f1e9;
  color: var(--d-green);
}
.shelf-card-arrow svg {
  transition: transform 160ms var(--ease-out);
}
.shelf-empty {
  border: 1px dashed #d6d9ca;
  border-radius: 16px;
  background: rgb(255 254 250 / 60%);
  padding: 52px 24px;
}
.shelf-empty-icon {
  width: 68px;
  height: 68px;
  border-radius: 20px;
  margin-bottom: 5px;
}
.shelf-empty > p {
  max-width: 360px;
  line-height: 1.8;
  overflow-wrap: anywhere;
}
.shelf-empty-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
  margin-top: 8px;
}
.shelf-external-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.shelf-external-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 44px;
}
.shelf-external-cover {
  width: 48px;
  height: 48px;
  flex-shrink: 0;
  object-fit: cover;
  border-radius: 8px;
  background: #edf0e8;
}
.shelf-external-list li > div {
  flex: 1;
  min-width: 0;
}
.shelf-external-list strong {
  display: block;
  overflow-wrap: anywhere;
}
.shelf-external-list span,
.shelf-external-credit,
.shelf-external-empty {
  color: var(--d-muted);
  font-size: 12px;
}
.shelf-external-credit {
  margin-top: 14px;
}
.shelf-external-credit a {
  color: var(--d-green);
}
@media (hover: hover) and (pointer: fine) {
  .shelf-card:hover {
    transform: translateY(-2px);
    border-color: #c9ceb9;
  }
  .shelf-card:hover .shelf-card-arrow svg {
    transform: translateX(2px);
  }
}
.shelf-card:active:not(:focus-visible) {
  transform: scale(0.985);
}
@media (max-width: 1050px) {
  .shelf-toolbar {
    flex-wrap: wrap;
    gap: 12px 18px;
  }
  .shelf-count {
    flex-basis: 100%;
  }
  .shelf-search {
    max-width: none;
  }
}
@media (min-width: 761px) and (max-width: 1050px) {
  .shelf-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 18px;
  }
}
@media (max-width: 700px) {
  .shelf-heading {
    flex-wrap: wrap;
    gap: 16px;
  }
  .shelf-toolbar {
    padding: 15px;
    gap: 12px;
  }
  .shelf-count strong {
    font-size: 26px;
  }
  .shelf-search {
    flex-basis: 100%;
  }
  .shelf-external-search {
    margin-left: auto;
    min-height: 36px;
  }
  .shelf-caption {
    margin-top: 22px;
    font-size: 10px;
  }
  .shelf-caption > span {
    display: none;
  }
  .shelf-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
  }
  .shelf-card {
    padding: 7px;
    border-radius: 11px;
  }
  .shelf-cover,
  .shelf-photo {
    aspect-ratio: 0.95;
  }
  .shelf-cover-frame {
    inset: 10px 8px 10px 14px;
  }
  .shelf-cover-label {
    top: 20px;
    font-size: 6px;
    letter-spacing: 1.1px;
  }
  .shelf-cover-letter {
    font-size: 54px;
  }
  .shelf-cover-note {
    left: 19px;
    bottom: 19px;
    font-size: 8px;
  }
  .shelf-cover-mark {
    display: none;
  }
  .shelf-card-body {
    padding: 12px 3px 4px;
  }
  .shelf-card h2 {
    font-size: 14px;
  }
  .shelf-card footer {
    gap: 4px;
    padding-top: 12px;
  }
  .shelf-activity {
    font-size: 10px;
  }
  .shelf-activity small {
    font-size: 8px;
  }
  .shelf-card-arrow {
    width: 24px;
    height: 24px;
  }
}
@media (max-width: 360px) {
  .shelf-grid {
    gap: 10px;
  }
  .shelf-card-arrow {
    display: none;
  }
  .shelf-cover-letter {
    font-size: 46px;
  }
  .shelf-cover-label {
    letter-spacing: 0.6px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .shelf-card,
  .shelf-card-arrow svg {
    transition: none;
  }
  .shelf-card:hover,
  .shelf-card:active:not(:focus-visible),
  .shelf-card:hover .shelf-card-arrow svg {
    transform: none;
  }
}
</style>
