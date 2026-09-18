<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import RoundForm from '@/components/rounds/RoundForm.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { useSession } from '@/stores/session'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'
import { useRoundFormActions } from '@/composables/useRoundFormActions'
import type { Round } from '@/types/journal'
import { useRound } from '@/composables/useRound'
import { updateRound } from '@/api/round'

const route = useRoute(); const router = useRouter()
const { user } = useSession()
const { groupId, snapshot } = useGroupContext()
const { busy, error, run } = useGroupOperation()
const { addItem, upload, notice } = useRoundFormActions()
const saved = ref<Round | null>(null)

const id = String(route.params.id)
const { current, loading, error: loadError, load } = useRound(id)
function save(round: Round, key: string) { return run(async () => { saved.value = await updateRound(groupId, id, round, key); await router.push(`/rounds/${saved.value.id}`) }) }
</script>

<template>
<RequestStatus :loading="loading" :error="loadError" @retry="load" /><p v-if="notice" class="j-notice" role="status">{{ notice }}</p><RoundForm v-if="snapshot && user && current" :snapshot="snapshot" :user="user" :editing="current" :saving="busy" :error="error" :saved="saved" :add-item="addItem" :upload-photo="upload" @submit="save" @again="saved = null" />
</template>
