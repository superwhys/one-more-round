<script setup lang="ts">
import { ref } from 'vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useRoundHistory } from '@/composables/useRoundHistory'
import RequestStatus from '@/components/common/RequestStatus.vue'
import RoundOverview from '@/components/rounds/RoundOverview.vue'
import RoundHistory from '@/components/rounds/RoundHistory.vue'
import GroupInviteDialog from '@/components/groups/GroupInviteDialog.vue'

const { groupId, snapshot, owner, handleAccessError } = useGroupContext()
const { page, loading, error, load, apply } = useRoundHistory()
const inviteOpen = ref(false)
</script>

<template>
<template v-if="snapshot"><header class="d-page-heading"><div><p class="d-eyebrow">{{ 'GOOD TIMES, TOGETHER' }}</p><h1>{{ '一起玩过的日子' }}<span class="d-title-dot">。</span></h1><p>输赢是一时，相聚值得记很久。</p></div></header><RequestStatus :loading="loading" :error="error" @retry="load()" /><RoundOverview :page="page" />
<RoundHistory :snapshot="snapshot" :page="page" :loading="loading" :error="error" :owner="owner" @filter="apply" @more="load(true)" @invite="inviteOpen = true" />
<GroupInviteDialog v-if="inviteOpen" :group-id="groupId" @close="inviteOpen = false" @access-error="handleAccessError" />
</template>
</template>
