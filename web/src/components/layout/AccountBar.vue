<script setup lang="ts">
import type { Group, User } from '@/types/journal'
defineProps<{ user: User; groups: Group[]; selected: string; hasGroup: boolean; busy: boolean }>()
defineEmits<{ select: [id: string]; setup: []; logout: [] }>()
</script>

<template>
<div class="j-account" :class="{ 'has-group': hasGroup }"><label>当前小组<select :value="selected" @change="$emit('select', ($event.target as HTMLSelectElement).value)"><option value="">创建 / 加入小组</option><option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option></select></label><span>{{ user.email }}</span><button @click="$emit('setup')">新小组</button><button :disabled="busy" @click="$emit('logout')">退出登录</button></div>
</template>
