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

const route = useRoute()
const id = String(route.params.id)
const { groupId, snapshot, owner, refresh, handleAccessError } = useGroupContext()
const { page, loading, error, load, apply } = useRoundHistory({ game: id })
const current = computed(() => snapshot.value?.games.find(item => item.id === id))
const inviteOpen = ref(false)

const name = ref('')
const { busy, error: saveError, notice, run } = useGroupOperation()
function rename() { return run(async () => { await manageGroup(groupId, 'alias', id, name.value); notice.value = '已保存'; try { await refresh() } catch { notice.value = '名称已保存，刷新失败，请重新加载' } }) }
</script>

<template>
<template v-if="snapshot"><RequestStatus :loading="loading" :error="error" @retry="load()" /><template v-if="current"><header class="d-page-heading"><div><p class="d-eyebrow">STORIES AT OUR TABLE</p><h1>{{ current.name }}<span class="d-title-dot">。</span></h1><p>输赢是一时，相聚值得记很久。</p></div><RouterLink :to="`/rounds/new?game=${current.id}`" class="d-button">＋ 再玩一局</RouterLink></header><RoundOverview :page="page" /><form class="j-inline" @submit.prevent="rename"><label class="d-field">本组中文名称<input v-model="name" :placeholder="current.name" required maxlength="255" /></label><button class="d-button secondary" :disabled="busy">保存名称</button></form><RequestStatus :error="saveError" :notice="notice" @retry="run(refresh)" @dismiss="notice = ''" /><RoundStatistics :snapshot="snapshot" :items="page.stats" />
<RoundHistory :snapshot="snapshot" :page="page" :loading="loading" :error="error" :owner="owner" fixed-game @filter="apply" @more="load(true)" @invite="inviteOpen = true" />
<GroupInviteDialog v-if="inviteOpen" :group-id="groupId" @close="inviteOpen = false" @access-error="handleAccessError" />
</template><div v-else class="d-empty"><h2>这页回忆不存在。</h2><RouterLink to="/">回到回顾</RouterLink></div></template>
</template>
