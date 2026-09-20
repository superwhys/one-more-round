<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { listNotifications, readNotification } from '@/api/notification'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import type { NotificationPage } from '@/types/journal'

const router = useRouter(); const session = useSession()
const page = ref<NotificationPage>({ items: [], unread: 0 }); const loading = ref(true); const error = ref('')
async function load() { loading.value = true; error.value = ''; try { page.value = await listNotifications() } catch (cause) { error.value = message(cause) } finally { loading.value = false } }
async function open(id: string, groupId: string, link: string) { const item = page.value.items.find(value => value.id === id); if (item && !item.read_at) { await readNotification(id); item.read_at = new Date().toISOString(); page.value.unread = Math.max(0, page.value.unread - 1); window.dispatchEvent(new Event('omr:notifications-changed')) } if (groupId && session.selected.value !== groupId) session.selectGroup(groupId); await router.push(link || '/') }
async function readAll() { for (const item of page.value.items.filter(value => !value.read_at)) await readNotification(item.id); page.value.items.forEach(item => { item.read_at ||= new Date().toISOString() }); page.value.unread = 0; window.dispatchEvent(new Event('omr:notifications-changed')) }
onMounted(load)
</script>
<template><header class="d-page-heading"><div><p class="d-eyebrow">AROUND OUR TABLE</p><h1>通知<span class="d-title-dot">。</span></h1><p>关联申请、新成员和即将到期的邀请都在这里。</p></div><button v-if="page.unread" class="d-text-link" @click="readAll">全部已读</button></header><RequestStatus :loading="loading" :error="error" @retry="load" /><section class="d-surface"><button v-for="item in page.items" :key="item.id" class="j-notification-row" :class="{ unread: !item.read_at }" @click="open(item.id, item.group_id, item.link)"><span class="j-notification-dot"></span><span><strong>{{ item.title }}</strong><small>{{ item.body }}</small><time>{{ new Date(item.created).toLocaleString('zh-CN') }}</time></span></button><div v-if="!page.items.length && !loading" class="d-empty"><h2>暂时没有新消息。</h2><p>下一次相聚正在路上。</p></div></section></template>
