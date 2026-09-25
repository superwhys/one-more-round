<script setup lang="ts">
import { computed, ref } from 'vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useRoundHistory } from '@/composables/useRoundHistory'
import RequestStatus from '@/components/common/RequestStatus.vue'
import RoundOverview from '@/components/rounds/RoundOverview.vue'
import RoundHistory from '@/components/rounds/RoundHistory.vue'
import GroupInviteDialog from '@/components/groups/GroupInviteDialog.vue'

import { useRoute, RouterLink } from 'vue-router'
import RoundStatistics from '@/components/rounds/RoundStatistics.vue'

const route = useRoute()
const id = String(route.params.id)
const { groupId, snapshot, owner, handleAccessError } = useGroupContext()
const { page, loading, error, load, apply } = useRoundHistory({ player: id })
const current = computed(() => snapshot.value?.players.find(item => item.id === id))
const inviteOpen = ref(false)
// commonGames 按当前筛选范围内的完整对局汇总排序，不受历史列表分页影响。
const commonGames = computed(() =>
  (snapshot.value?.games ?? [])
    .map(game => ({ id: game.id, name: game.name, count: page.value.activity[game.id]?.count ?? 0 }))
    .filter(game => game.count > 0)
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name, 'zh-CN'))
    .slice(0, 3),
)
</script>

<template>
  <template v-if="snapshot"
    ><RequestStatus :loading="loading" :error="error" @retry="load()" /><template v-if="current"
      ><header class="d-page-heading">
        <div>
          <p class="d-eyebrow">STORIES AT OUR TABLE</p>
          <h1>{{ current.name }}<span class="d-title-dot">。</span></h1>
          <p>输赢是一时，相聚值得记很久。</p>
        </div>
      </header>
      <RoundOverview :page="page" />
      <section v-if="!loading && !error" class="d-surface player-games">
        <h2>常玩的桌游</h2>
        <p>按当前筛选范围内的参与局数排序</p>
        <ul v-if="commonGames.length">
          <li v-for="game in commonGames" :key="game.id">
            <RouterLink :to="`/games/${game.id}`">{{ game.name }}</RouterLink>
            <span>{{ game.count }} 局</span>
          </li>
        </ul>
        <p v-else>当前筛选范围内还没有对局。</p>
      </section>
      <RoundStatistics :snapshot="snapshot" :items="page.stats" :player-id="id" />
      <RoundHistory
        :snapshot="snapshot"
        :page="page"
        :loading="loading"
        :error="error"
        :owner="owner"
        fixed-player
        @filter="apply"
        @more="load(true)"
        @invite="inviteOpen = true"
      />
      <GroupInviteDialog
        v-if="inviteOpen"
        :group-id="groupId"
        @close="inviteOpen = false"
        @access-error="handleAccessError"
      />
    </template>
    <div v-else class="d-empty">
      <h2>这页回忆不存在。</h2>
      <RouterLink to="/">回到回顾</RouterLink>
    </div></template
  >
</template>

<style scoped>
.player-games {
  margin-bottom: 24px;
}
.player-games h2 {
  margin: 0;
  font-size: 17px;
}
.player-games > p {
  margin: 8px 0 0;
  color: #838779;
  font-size: 12px;
}
.player-games ul {
  margin: 18px 0 0;
  padding: 0;
  list-style: none;
}
.player-games li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-top: 1px solid var(--d-line);
  font-size: 13px;
}
.player-games li a {
  min-width: 0;
  color: var(--d-accent);
  overflow-wrap: anywhere;
}
.player-games li a:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}
.player-games li span {
  flex-shrink: 0;
  color: #838779;
}
@media (max-width: 560px) {
  .player-games {
    padding: 18px;
  }
}
</style>
