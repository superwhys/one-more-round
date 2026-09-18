<script setup lang="ts">
import { useRoute } from 'vue-router'
import RecordEditor from '@/components/design/RecordEditor.vue'
const route = useRoute()
</script>

<template>
<RecordEditor :initial-game="typeof route.query.game === 'string' ? route.query.game : undefined" />
</template>
