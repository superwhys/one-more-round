import { createRouter, createWebHistory } from 'vue-router'
import { useSession } from '@/stores/session'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/share', name: 'public-round', component: () => import('@/views/rounds/PublicRoundView.vue'), meta: { title: '分享的对局', anonymous: true } },
    { path: '/', component: () => import('@/views/JournalLayoutView.vue'), children: [
      { path: 'login', name: 'login', component: () => import('@/views/auth/LoginView.vue'), meta: { title: '登录', public: true } },
      { path: 'join', name: 'group-setup', component: () => import('@/views/groups/GroupSetupView.vue'), meta: { title: '创建或加入小组' } },
      { path: '', component: () => import('@/views/groups/GroupLayoutView.vue'), meta: { requiresGroup: true }, children: [
        { path: '', name: 'review', component: () => import('@/views/review/ReviewView.vue'), meta: { title: '回顾', section: 'review' } },
        { path: 'recaps', name: 'recap', component: () => import('@/views/review/RecapView.vue'), meta: { title: '月度与年度回顾', section: 'review', back: true } },
        { path: 'notifications', name: 'notifications', component: () => import('@/views/notifications/NotificationView.vue'), meta: { title: '通知', section: '', back: true } },
        { path: 'games', name: 'game-list', component: () => import('@/views/games/GameListView.vue'), meta: { title: '桌游', section: 'games' } },
        { path: 'games/:id', name: 'game-detail', component: () => import('@/views/games/GameDetailView.vue'), meta: { title: '桌游详情', section: 'games', back: true } },
        { path: 'players/:id', name: 'player-detail', component: () => import('@/views/players/PlayerDetailView.vue'), meta: { title: '玩家详情', section: 'group', back: true } },
        { path: 'group', name: 'group', component: () => import('@/views/groups/GroupView.vue'), meta: { title: '小组', section: 'group' } },
        { path: 'recycle-bin', name: 'recycle-bin', component: () => import('@/views/groups/RecycleBinView.vue'), meta: { title: '回收站', section: 'group', back: true } },
        { path: 'rounds/new', name: 'round-create', component: () => import('@/views/rounds/RoundCreateView.vue'), meta: { title: '记一局', section: 'review', editor: true, back: true } },
        { path: 'rounds/:id', name: 'round-detail', component: () => import('@/views/rounds/RoundDetailView.vue'), meta: { title: '对局详情', section: 'review', back: true } },
        { path: 'edit/:id', name: 'round-edit', component: () => import('@/views/rounds/RoundEditView.vue'), meta: { title: '编辑对局', section: 'review', editor: true, back: true } },
        { path: ':pathMatch(.*)*', name: 'not-found', component: () => import('@/views/NotFoundView.vue'), meta: { title: '页面不存在' } },
      ] },
    ] },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async to => {
  const session = useSession()
  if (session.captureInvitation(to.path, to.hash)) {
    return { path: to.path, query: to.query, hash: '', replace: true }
  }
  if (to.meta.anonymous) return
  await session.initialize()
  if (!session.user.value && !to.meta.public) return { name: 'login', replace: true }
  if (session.user.value && to.meta.public) return { name: session.joinToken.value || !session.selected.value ? 'group-setup' : 'review', replace: true }
  if (to.meta.requiresGroup && !session.selected.value) return { name: 'group-setup', replace: true }
})
router.afterEach(to => { document.title = `${String(to.meta.title ?? '对局日记')} · 又一局` })
