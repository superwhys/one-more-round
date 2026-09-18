<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { players, rounds, demoSession } from '@/stores/designPreview'
import { useDesignPreview } from '@/composables/useDesignPreview'
import Icon from '@/components/common/AppIcon.vue'
import Avatar from '@/components/design/PlayerAvatar.vue'
const { modal, newGroup } = useDesignPreview()
</script>

<template>
<header class="d-page-heading"><div><p class="d-eyebrow">OUR LITTLE CIRCLE</p><h1>{{ demoSession.group }}<span class="d-title-dot">。</span></h1><p>桌子不大，刚好坐得下这些老朋友。</p></div><button class="d-button" @click="modal = 'invite'"><Icon name="plus" :size="17" />邀请朋友</button></header>
          <div class="d-group-banner"><div class="d-group-banner-art">五<span>FRIDAY CLUB</span></div><div><span class="d-pill">私密小组</span><h2>有空就聚，再来一局。</h2><p>{{ players.length }} 位玩家档案 · {{ players.filter((p) => p.linked).length }} 位账号成员 · {{ rounds.length }} 段共同回忆</p></div><button class="d-icon-button" aria-label="修改演示小组名称" @click="modal = 'group'; newGroup = demoSession.group"><Icon name="edit" /></button></div>
          <div class="d-group-columns"><section class="d-surface"><div class="d-list-heading"><h2>同桌的朋友<small>玩家档案</small></h2><RouterLink to="/design/new" class="d-text-link">添加玩家<Icon name="plus" :size="15" /></RouterLink></div><p class="d-note">参加过就是朋友。只有昵称，也能留下战绩。</p><RouterLink v-for="player in players" :key="player.id" :to="`/design/players/${player.id}`" class="d-member-row"><Avatar :id="player.id" /><div><strong>{{ player.name }}{{ player.id === 'lin' ? '（我）' : '' }}</strong><small>{{ player.linked ? '已关联账号' : '昵称玩家 · 暂未关联账号' }}</small></div><span>{{ rounds.filter((r) => r.players.includes(player.id)).length }} 局</span><Icon name="arrow" :size="16" /></RouterLink></section><section class="d-surface"><div class="d-list-heading"><h2>能一起回顾的人<small>账号成员</small></h2></div><div v-for="player in players.filter((p) => p.linked)" :key="player.id" class="d-member-row"><Avatar :id="player.id" small /><strong>{{ player.name }}</strong><span class="d-pill">{{ player.id === 'lin' ? '组主' : '成员' }}</span></div><div class="d-association-note"><Icon name="link" /><h3>以前的对局，也是我的</h3><p>新朋友登录后，可申请关联已有玩家档案，由组主确认。</p><button class="d-button secondary full" @click="modal = 'association'">体验玩家关联</button></div></section></div>
</template>
