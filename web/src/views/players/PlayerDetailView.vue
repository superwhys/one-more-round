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
      <RoundOverview :page="page" /><RoundStatistics :snapshot="snapshot" :items="page.stats" :player-id="id" />
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
