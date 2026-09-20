<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import RoundForm from '@/components/rounds/RoundForm.vue'
import { useSession } from '@/stores/session'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'
import { useRoundFormActions } from '@/composables/useRoundFormActions'
import type { Round } from '@/types/journal'
import { createRound } from '@/api/round'

const route = useRoute()
const { user } = useSession()
const { groupId, snapshot } = useGroupContext()
const { busy, error, run } = useGroupOperation()
const { addItem, upload, notice } = useRoundFormActions()
const saved = ref<Round | null>(null)
function save(round: Round, key: string) {
  return run(async () => {
    saved.value = await createRound(groupId, round, key)
  })
}
</script>

<template>
  <p v-if="notice" class="j-notice" role="status">{{ notice }}</p>
  <RoundForm
    v-if="snapshot && user"
    :snapshot="snapshot"
    :user="user"
    :initial-game="typeof route.query.game === 'string' ? route.query.game : undefined"
    :saving="busy"
    :error="error"
    :saved="saved"
    :add-item="addItem"
    :upload-photo="upload"
    @submit="save"
    @again="saved = null"
  />
</template>
