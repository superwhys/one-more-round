<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/common/AppIcon.vue'
import { useGroupContext } from '@/composables/useGroupContext'
import { useRoundHistory } from '@/composables/useRoundHistory'
import RequestStatus from '@/components/common/RequestStatus.vue'
import RoundOverview from '@/components/rounds/RoundOverview.vue'
import RoundHistory from '@/components/rounds/RoundHistory.vue'
import GroupInviteDialog from '@/components/groups/GroupInviteDialog.vue'

const { groupId, snapshot, owner, handleAccessError } = useGroupContext()
const { page, loading, error, load, apply } = useRoundHistory()
const inviteOpen = ref(false)
</script>

<template>
  <template v-if="snapshot">
    <header class="d-page-heading review-cover">
      <div class="review-intro">
        <p class="d-eyebrow"><span></span>GOOD TIMES, TOGETHER</p>
        <h1>一起玩过的日子<span class="d-title-dot">。</span></h1>
        <p class="review-description">把一桌欢笑、一场胜利，<br />和那句「再来一局」，都留在这里。</p>
        <div class="review-actions">
          <RouterLink to="/rounds/new" class="d-button"><Icon name="plus" :size="18" />记下这一局</RouterLink>
          <RouterLink to="/recaps" class="review-recap"
            ><Icon name="calendar" :size="16" />月度 / 年度回顾<Icon name="arrow" :size="16"
          /></RouterLink>
          <RouterLink to="/album" class="review-recap"
            ><Icon name="photo" :size="16" />聚会相册<Icon name="arrow" :size="16"
          /></RouterLink>
        </div>
      </div>
      <div class="review-keepsake" aria-hidden="true">
        <div class="keepsake-card">
          <span class="keepsake-label">OUR LITTLE COLLECTION</span>
          <div class="keepsake-table">
            <Icon name="game" :size="54" /><span>＋</span><Icon name="heart" :size="42" />
          </div>
          <strong>好时光，再来一局。</strong>
          <span class="keepsake-footnote">和喜欢的人，玩喜欢的游戏</span>
        </div>
        <span class="keepsake-stamp">相聚<br />值得记录</span>
      </div>
    </header>
    <RequestStatus :loading="loading" :error="error" @retry="load()" /><RoundOverview :page="page" />
    <RoundHistory
      :snapshot="snapshot"
      :page="page"
      :loading="loading"
      :error="error"
      :owner="owner"
      @filter="apply"
      @more="load(true)"
      @invite="inviteOpen = true"
    />
    <GroupInviteDialog
      v-if="inviteOpen"
      :group-id="groupId"
      @close="inviteOpen = false"
      @access-error="handleAccessError"
    />
  </template>
</template>

<style scoped>
.review-cover {
  position: relative;
  isolation: isolate;
  min-height: 268px;
  padding: 32px 36px;
  border: 1px solid #e8ded0;
  border-radius: 18px;
  background: #f0e9dc;
  gap: 28px;
}
.review-intro {
  position: relative;
  z-index: 1;
  min-width: 0;
}
.review-intro .d-eyebrow {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #777b63;
  letter-spacing: 1.8px;
}
.review-intro .d-eyebrow span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--d-accent);
}
.review-cover h1 {
  font-size: clamp(27px, 2.6vw, 38px);
  letter-spacing: -0.6px;
}
.review-description {
  margin-top: 12px !important;
  color: #777b69;
  font-size: 13px;
  line-height: 1.9;
}
.review-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 20px;
  margin-top: 23px;
}
.review-recap {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 44px;
  color: #6c725e;
  font-size: 11px;
}
.review-recap > svg:last-child {
  transition: transform 160ms var(--ease-out);
}
.review-keepsake {
  position: relative;
  flex: 0 0 225px;
  padding: 8px 8px 8px 0;
}
.keepsake-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 19px 14px 22px;
  border: 1px solid #dfddcd;
  border-radius: 3px;
  background: #fffdf4;
  box-shadow: 0 12px 24px -16px #57472b50;
  transform: rotate(-7deg);
}
.keepsake-card::before {
  content: '';
  position: absolute;
  top: -9px;
  left: calc(50% - 27px);
  width: 54px;
  height: 20px;
  background: #a8b09580;
  transform: rotate(4deg);
}
.keepsake-label {
  color: #969580;
  font-size: 7px;
  letter-spacing: 1.8px;
}
.keepsake-table {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 15px;
  width: 100%;
  margin: 13px 0 17px;
  padding: 24px 8px;
  background: #e5e9db;
  color: #76876a;
}
.keepsake-table > svg:first-child {
  transform: rotate(-12deg);
  color: #bc684d;
}
.keepsake-table > svg:last-child {
  transform: rotate(10deg);
}
.keepsake-card strong {
  color: #656b55;
  font:
    19px 'Songti SC',
    serif;
}
.keepsake-footnote {
  margin-top: 8px;
  color: #969780;
  font-size: 8px;
  letter-spacing: 1px;
}
.keepsake-stamp {
  position: absolute;
  right: -10px;
  bottom: 1px;
  display: grid;
  place-content: center;
  width: 62px;
  height: 62px;
  border: 1px dashed #bc745c;
  border-radius: 50%;
  background: #f2e8dcee;
  color: #a56650;
  font-size: 10px;
  text-align: center;
  transform: rotate(12deg);
}
@media (hover: hover) and (pointer: fine) {
  .review-recap:hover > svg:last-child {
    transform: translateX(3px);
  }
}
@media (max-width: 1150px) {
  .review-cover {
    padding: 28px;
  }
  .review-keepsake {
    flex-basis: 180px;
  }
  .keepsake-table {
    gap: 7px;
    padding: 19px 4px;
  }
}
@media (max-width: 960px) {
  .review-keepsake {
    display: none;
  }
}
@media (max-width: 760px) {
  .review-cover {
    min-height: 0;
    padding: 25px 23px;
    border-radius: 14px;
  }
  .review-cover h1 {
    font-size: 28px;
  }
  .review-intro .d-eyebrow {
    font-size: 8px;
    letter-spacing: 1.3px;
  }
  .review-description {
    font-size: 12px;
  }
  .review-actions {
    margin-top: 19px;
  }
  .review-actions .d-button {
    display: none;
  }
}
@media (max-width: 360px) {
  .review-cover {
    padding: 22px 18px;
  }
  .review-cover h1 {
    font-size: 25px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .review-recap:hover > svg:last-child {
    transform: none;
  }
}
</style>
