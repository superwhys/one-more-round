<script setup lang="ts">
import { ref } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { players, rounds, demoSession, playerName } from '@/stores/designPreview'
import { provideDesignPreview } from '@/composables/useDesignPreview'
import Icon from '@/components/common/AppIcon.vue'
import Modal from '@/components/common/AppModal.vue'
import DesignLayout from '@/components/layout/DesignLayout.vue'

import '@/styles/theme.css'
const route = useRoute(); const router = useRouter()
const { modal, toast, newGroup, notify } = provideDesignPreview()
const notice = ref('')
const email = ref('')
const otp = ref('')
const sentCode = ref(false)
const associationId = ref('')
const associationPending = ref(false)
function sendCode() { if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value)) { notice.value = '填写一个完整的邮箱地址吧'; return }; sentCode.value = true; notice.value = '这是演示，不会发送邮件。输入 123456 即可体验。' }
function demoLogin() { if (otp.value !== '123456') { notice.value = '演示验证码是 123456'; return }; demoSession.joined = true; modal.value = ''; notify('欢迎来到「周五不散场」· 演示加入成功') }
function renameGroup() { if (!newGroup.value.trim()) return; demoSession.group = newGroup.value.trim(); modal.value = ''; notify('演示小组名称已更新') }
function welcome() { modal.value = 'welcome'; notice.value = ''; sentCode.value = false }
</script>

<template>
<div class="design-prototype">
<DesignLayout :group-name="demoSession.group" :round-count="rounds.length" :is-editor="!!route.meta.editor" :section="String(route.meta.section ?? 'review')" :back="!!route.meta.back" :title="String(route.meta.title)" @group="modal = 'group'; newGroup = demoSession.group" @guide="modal = 'guide'" @welcome="welcome" @home="router.push('/design')">
  <RouterView v-slot="{ Component }"><component :is="Component" :key="route.fullPath" /></RouterView>
</DesignLayout>
<div v-if="toast" class="d-toast" role="status"><Icon name="check" :size="17" />{{ toast }}</div>
<Modal v-if="modal === 'guide'" title="先逛逛，再记一局。" @close="modal = ''"><p class="d-modal-copy">这是一版可点击产品稿。所有人物和记录都是示例，操作仅在本次预览中生效。</p><ol class="d-guide-list"><li><strong>看看一起玩过的日子</strong><span>点一张时间线卡片，查看结果、感想和照片。</span></li><li><strong>试着记一局</strong><span>选游戏、选玩家、填结果；支持三种模式、共同获胜与未记结果。</span></li><li><strong>保存，再回头看看</strong><span>演示记录会进入时间线、游戏和玩家详情；也可以编辑、删除。</span></li></ol><button class="d-button full" @click="modal = ''; router.push('/design/new')">体验记一局<Icon name="arrow" :size="17" /></button><p class="d-note">刷新页面会重置演示数据。登录、邀请和图片仅模拟交互，不发送邮件、不上传文件、不生成真实邀请。</p></Modal><Modal v-if="modal === 'invite'" title="给朋友留个位置。" @close="modal = ''"><p class="d-modal-copy">加入「{{ demoSession.group }}」后，就能一起回顾以前的对局。</p><div v-if="demoSession.invited" class="d-invite-preview"><Icon name="link" /><strong>小组邀请已生成（演示）</strong><code>one-more-round.example/join/friday-demo</code><p>此地址仅展示邀请样式，不是真实可用链接。</p></div><button v-else class="d-button full" @click="demoSession.invited = true">生成演示邀请</button><button v-if="demoSession.invited" class="d-button secondary full" @click="demoSession.invited = false; notify('演示邀请已撤销')">撤销这份邀请</button><p class="d-note">邀请只用于加入小组，不直接公开对局或照片。</p></Modal><Modal v-if="modal === 'group'" title="我们的这张桌子" @close="modal = ''"><p class="d-modal-copy">当前演示只有一个小组。正式版本中，可在这里切换自己加入的小组。</p><label class="d-field">小组名称<input v-model="newGroup" maxlength="40" @keyup.enter="renameGroup" /></label><button class="d-button full" @click="renameGroup">保存演示名称</button></Modal><Modal v-if="modal === 'welcome'" title="这桌，就等你了。" @close="modal = ''"><div class="d-welcome-invite"><span class="d-group-glyph">五</span><div><strong>周五不散场</strong><small>你收到了一份试用与小组邀请 · 演示</small></div></div><p class="d-modal-copy">用邮箱登录，加入朋友们的共同对局日记。</p><label class="d-field">邮箱<input v-model="email" type="email" placeholder="you@example.com" /></label><button v-if="!sentCode" class="d-button full" @click="sendCode">获取演示验证码</button><template v-else><label class="d-field">验证码<input v-model="otp" inputmode="numeric" maxlength="6" placeholder="演示请输入 123456" @keyup.enter="demoLogin" /></label><button class="d-button full" @click="demoLogin">登录并加入小组</button></template><p v-if="notice" class="d-note" role="status">{{ notice }}</p><p class="d-note">仅模拟登录体验，不验证真实邮箱，也不创建账号。</p></Modal><Modal v-if="modal === 'association'" title="把以前的对局，关联回来。" @close="modal = ''"><template v-if="!associationPending"><p class="d-modal-copy">演示一位新成员申请关联玩家档案。昵称相同也不会自动认领。</p><label class="d-field">选择你的玩家档案<select v-model="associationId"><option value="">请选择</option><option v-for="player in players.filter((p) => !p.linked)" :key="player.id" :value="player.id">{{ player.name }}</option></select></label><button class="d-button full" :disabled="!associationId" @click="associationPending = true">提交演示申请</button></template><div v-else class="d-empty"><Icon name="clock" :size="32" /><h3>等待组主确认</h3><p>确认关联「{{ playerName(associationId) }}」后，历史参与记录仍然保留，不重复创建玩家。</p></div></Modal></div>
</template>
