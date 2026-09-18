import type { RouteRecordRaw } from 'vue-router'
export const designRoutes: RouteRecordRaw[] = [
  { path: '/design', component: () => import('@/views/design/DesignLayoutView.vue'), meta: { design: true }, children: [
    { path: '', name: 'design-review', component: () => import('@/views/design/DesignReviewView.vue'), meta: { title: '产品稿', section: 'review' } },
    { path: 'games', name: 'design-games', component: () => import('@/views/design/DesignGameListView.vue'), meta: { title: '桌游产品稿', section: 'games' } },
    { path: 'games/:id', name: 'design-game-detail', component: () => import('@/views/design/DesignGameDetailView.vue'), meta: { title: '桌游详情', section: 'games', back: true } },
    { path: 'group', name: 'design-group', component: () => import('@/views/design/DesignGroupView.vue'), meta: { title: '小组产品稿', section: 'group' } },
    { path: 'players/:id', name: 'design-player-detail', component: () => import('@/views/design/DesignPlayerDetailView.vue'), meta: { title: '玩家档案', section: 'group', back: true } },
    { path: 'rounds/:id', name: 'design-round-detail', component: () => import('@/views/design/DesignRoundDetailView.vue'), meta: { title: '对局回忆', section: 'review', back: true } },
    { path: 'new', name: 'design-round-create', component: () => import('@/views/design/DesignRoundCreateView.vue'), meta: { title: '记一局', section: 'review', editor: true, back: true } },
    { path: 'edit/:id', name: 'design-round-edit', component: () => import('@/views/design/DesignRoundEditView.vue'), meta: { title: '编辑对局', section: 'review', editor: true, back: true } },
    { path: ':pathMatch(.*)*', name: 'design-not-found', component: () => import('@/views/design/DesignNotFoundView.vue'), meta: { title: '页面不存在' } },
  ] },
]
