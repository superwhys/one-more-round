<script setup lang="ts">
import { manageGroup } from '@/api/group'
import { useGroupOperation } from '@/composables/useGroupOperation'

import { computed, ref } from 'vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useRoundHistory } from '@/composables/useRoundHistory'
import RequestStatus from '@/components/common/RequestStatus.vue'
import RoundOverview from '@/components/rounds/RoundOverview.vue'
import RoundHistory from '@/components/rounds/RoundHistory.vue'
import GroupInviteDialog from '@/components/groups/GroupInviteDialog.vue'

import { useRoute, RouterLink } from 'vue-router'
import RoundStatistics from '@/components/rounds/RoundStatistics.vue'
import Icon from '@/components/common/AppIcon.vue'

const route = useRoute()
const id = String(route.params.id)
const { groupId, snapshot, owner, refresh, handleAccessError } = useGroupContext()
const { page, loading, error, load, apply } = useRoundHistory({ game: id })
const current = computed(() => snapshot.value?.games.find(item => item.id === id))
const inviteOpen = ref(false)

const name = ref('')
const { busy, error: saveError, notice, run } = useGroupOperation()
function rename() {
  return run(async () => {
    await manageGroup(groupId, 'alias', id, name.value)
    notice.value = '已保存'
    try {
      await refresh()
    } catch {
      notice.value = '名称已保存，刷新失败，请重新加载'
    }
  })
}
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
        <RouterLink :to="`/rounds/new?game=${current.id}`" class="d-button">＋ 再玩一局</RouterLink>
      </header>
      <section class="d-surface game-profile" aria-label="桌游资料">
        <img v-if="current.cover" class="game-profile-cover" :src="current.cover" :alt="`${current.name} 封面`" />
        <div
          v-else
          class="game-profile-cover game-profile-placeholder"
          role="img"
          :aria-label="`${current.name} 暂无封面`"
        >
          <Icon name="game" :size="32" aria-hidden="true" />
          <span>暂无封面</span>
        </div>
        <div class="game-profile-info">
          <h2>桌游资料</h2>
          <dl>
            <div>
              <dt>本组名称</dt>
              <dd>{{ current.name }}</dd>
            </div>
            <div v-if="current.original">
              <dt>BGG 原名</dt>
              <dd>{{ current.original }}</dd>
            </div>
            <div>
              <dt>资料来源</dt>
              <dd>
                <a
                  v-if="current.bgg_id"
                  :href="`https://boardgamegeek.com/boardgame/${current.bgg_id}`"
                  target="_blank"
                  rel="noopener noreferrer"
                  >BoardGameGeek ↗</a
                >
                <span v-else>本组手动添加</span>
              </dd>
            </div>
          </dl>
        </div>
      </section>
      <RoundOverview :page="page" />
      <form class="j-inline" @submit.prevent="rename">
        <label class="d-field"
          >本组中文名称<input v-model="name" :placeholder="current.name" required maxlength="255" /></label
        ><button class="d-button secondary" :disabled="busy">保存名称</button>
      </form>
      <RequestStatus :error="saveError" :notice="notice" @retry="run(refresh)" @dismiss="notice = ''" /><RoundStatistics
        :snapshot="snapshot"
        :items="page.stats"
      />
      <RoundHistory
        :snapshot="snapshot"
        :page="page"
        :loading="loading"
        :error="error"
        :owner="owner"
        fixed-game
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
.game-profile {
  display: flex;
  align-items: center;
  gap: 26px;
  margin-bottom: 24px;
}
.game-profile-cover {
  width: 130px;
  height: 156px;
  flex: 0 0 130px;
  border-radius: 8px;
  object-fit: contain;
  background: #f0eee5;
}
.game-profile-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #8c957e;
  font-size: 11px;
}
.game-profile-info {
  min-width: 0;
}
.game-profile-info h2 {
  margin: 0 0 18px;
  font-size: 17px;
}
.game-profile-info dl {
  display: grid;
  gap: 12px;
  margin: 0;
}
.game-profile-info dl > div {
  display: flex;
  gap: 20px;
}
.game-profile-info dt {
  min-width: 68px;
  color: #838779;
  font-size: 12px;
}
.game-profile-info dd {
  min-width: 0;
  margin: 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.game-profile-info a {
  color: var(--d-accent);
  text-decoration: underline;
  text-underline-offset: 3px;
}
@media (max-width: 560px) {
  .d-page-heading {
    flex-wrap: wrap;
  }
  .d-page-heading > div {
    min-width: 0;
  }
  .d-page-heading > .d-button {
    width: 100%;
  }
  .game-profile {
    align-items: flex-start;
    gap: 16px;
    padding: 16px;
  }
  .game-profile-cover {
    width: 90px;
    height: 112px;
    flex-basis: 90px;
  }
  .game-profile-info dl > div {
    display: block;
  }
  .game-profile-info dt {
    margin-bottom: 3px;
  }
}
</style>
