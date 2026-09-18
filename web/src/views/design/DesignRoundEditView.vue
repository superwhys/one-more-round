<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { rounds } from '@/stores/designPreview'
import RecordEditor from '@/components/design/RecordEditor.vue'
const route = useRoute()
const itemId = computed(() => String(route.params.id))
const editing = computed(() => rounds.find(round => round.id === itemId.value))
</script>

<template>
<RecordEditor v-if="editing" :editing="editing" /><div v-else class="d-empty">这页回忆不存在。</div>
</template>
