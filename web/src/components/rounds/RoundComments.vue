<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Modal from '@/components/common/AppModal.vue'
import PlayerChip from '@/components/common/PlayerChip.vue'
import RequestStatus from '@/components/common/RequestStatus.vue'
import { createRoundComment, deleteRoundComment, listRoundComments } from '@/api/round'
import { useGroupContext } from '@/composables/useGroupContext'
import { useGroupOperation } from '@/composables/useGroupOperation'
import { useSession } from '@/stores/session'
import { message } from '@/utils/error'
import type { RoundComment } from '@/types/journal'

const props = defineProps<{ roundId: string }>()
const { user } = useSession()
const { groupId, owner, commenterName, handleAccessError } = useGroupContext()
const { busy, error, run } = useGroupOperation()
const comments = ref<RoundComment[]>([])
const loading = ref(true)
const loadError = ref('')
const draft = ref('')
const replyTo = ref<RoundComment | null>(null)
const key = ref(crypto.randomUUID())
const pendingDelete = ref<RoundComment | null>(null)
const count = computed(() => Array.from(draft.value).length)
const canSubmit = computed(() => draft.value.trim() !== '' && count.value <= 500 && !busy.value)

function byCreated(left: RoundComment, right: RoundComment) {
  if (left.created === right.created) return left.id.localeCompare(right.id)
  return left.created < right.created ? -1 : 1
}

const roots = computed(() =>
  comments.value
    .filter(comment => !comment.parent_id)
    .slice()
    .sort(byCreated),
)

function repliesOf(id: string) {
  return comments.value
    .filter(comment => comment.parent_id === id)
    .slice()
    .sort(byCreated)
}

function canDelete(comment: RoundComment) {
  return owner.value || comment.author === user.value?.id
}

function beijingTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    comments.value = (await listRoundComments(groupId, props.roundId)).items.slice().sort(byCreated)
  } catch (cause) {
    loadError.value = message(cause)
    await handleAccessError(cause)
  } finally {
    loading.value = false
  }
}

function startReply(comment: RoundComment) {
  replyTo.value = comment
}

function cancelReply() {
  replyTo.value = null
}

function submit() {
  if (!canSubmit.value) return
  const body = draft.value.trim()
  const parentID = replyTo.value?.id
  const submission = key.value
  return run(async () => {
    const saved = await createRoundComment(groupId, props.roundId, { body, parent_id: parentID ?? null }, submission)
    comments.value = [...comments.value.filter(item => item.id !== saved.id), saved].sort(byCreated)
    draft.value = ''
    replyTo.value = null
    key.value = crypto.randomUUID()
  })
}

function confirmDelete(comment: RoundComment) {
  if (!canDelete(comment)) return
  pendingDelete.value = comment
}

function removeComment() {
  const target = pendingDelete.value
  if (!target) return
  return run(async () => {
    await deleteRoundComment(groupId, props.roundId, target.id)
    comments.value = comments.value.filter(item => item.id !== target.id && item.parent_id !== target.id)
    pendingDelete.value = null
  })
}

onMounted(load)
</script>

<template>
  <section class="j-comments" aria-labelledby="round-comments-title">
    <h3 id="round-comments-title">评论</h3>
    <RequestStatus :loading="loading" :error="loadError || error" @retry="load" />
    <div v-if="!loading && !loadError && !comments.length" class="d-empty">
      <h2>还没有评论，写一句给这局。</h2>
    </div>
    <article v-for="comment in roots" :key="comment.id" class="j-comment">
      <header>
        <PlayerChip :name="commenterName(comment.author)" :me="comment.author === user?.id" />
        <time>{{ beijingTime(comment.created) }}</time>
      </header>
      <p>{{ comment.body }}</p>
      <div class="j-comment-actions">
        <button type="button" class="d-text-link" @click="startReply(comment)">回复</button>
        <button v-if="canDelete(comment)" type="button" class="d-text-link" @click="confirmDelete(comment)">
          删除
        </button>
      </div>
      <article v-for="reply in repliesOf(comment.id)" :key="reply.id" class="j-comment nested">
        <header>
          <PlayerChip :name="commenterName(reply.author)" :me="reply.author === user?.id" />
          <time>{{ beijingTime(reply.created) }}</time>
        </header>
        <p>{{ reply.body }}</p>
        <div class="j-comment-actions">
          <button v-if="canDelete(reply)" type="button" class="d-text-link" @click="confirmDelete(reply)">删除</button>
        </div>
      </article>
    </article>
    <form class="j-comment-form" @submit.prevent="submit">
      <p v-if="replyTo" class="d-note">
        回复 {{ commenterName(replyTo.author) }}
        <button type="button" class="d-text-link" @click="cancelReply">取消</button>
      </p>
      <label class="d-field"
        >{{ replyTo ? '写一句回复' : '写一句给这局' }}
        <textarea
          v-model="draft"
          rows="3"
          maxlength="2000"
          :placeholder="replyTo ? '回复这条评论' : '留下一句桌边闲话'"
        />
        <small>{{ count }} / 500 字</small>
      </label>
      <button class="d-button" type="submit" :disabled="!canSubmit">{{ replyTo ? '发送回复' : '发表评论' }}</button>
    </form>
  </section>
  <Modal v-if="pendingDelete" title="删除这条评论？" @close="pendingDelete = null">
    <p>删除后无法恢复。如果这是一条对局评论，它下面的回复也会一起消失。</p>
    <p v-if="error" class="j-error">{{ error }}</p>
    <div class="d-modal-actions">
      <button class="d-button secondary" type="button" @click="pendingDelete = null">再想想</button>
      <button class="d-button" type="button" :disabled="busy" @click="removeComment">删除评论</button>
    </div>
  </Modal>
</template>
