<script setup lang="ts">
import { nextTick, onMounted, onBeforeUnmount, ref } from 'vue'
import Icon from './AppIcon.vue'
defineProps<{ title: string; wide?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement>()
const animateEntry = ref(false)
let previousFocus: HTMLElement | null = null
onMounted(() => {
  previousFocus = document.activeElement as HTMLElement
  animateEntry.value = !previousFocus?.matches(':focus-visible')
  void nextTick(() => dialog.value?.showModal())
})
onBeforeUnmount(() => {
  dialog.value?.close()
  previousFocus?.focus()
})
</script>
<template>
  <dialog
    ref="dialog"
    class="d-modal"
    :class="{ wide, 'modal-motion': animateEntry }"
    aria-labelledby="design-dialog-title"
    @cancel.prevent="emit('close')"
    @click="
      event => {
        if (event.target === dialog) emit('close')
      }
    "
  >
    <header>
      <div>
        <p class="d-eyebrow">ONE MORE ROUND</p>
        <h2 id="design-dialog-title">{{ title }}</h2>
      </div>
      <button class="d-icon-button" aria-label="关闭弹窗" @click="emit('close')"><Icon name="close" /></button>
    </header>
    <slot />
  </dialog>
</template>

<style scoped>
.modal-motion[open] {
  opacity: 1;
  transform: scale(1);
  transition:
    opacity 200ms var(--ease-out),
    transform 200ms var(--ease-out);
}
.modal-motion[open]::backdrop {
  opacity: 1;
  transition: opacity 200ms var(--ease-out);
}
@starting-style {
  .modal-motion[open] {
    opacity: 0;
    transform: scale(0.97);
  }
  .modal-motion[open]::backdrop {
    opacity: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .modal-motion[open] {
    transform: none;
  }
  @starting-style {
    .modal-motion[open] {
      transform: none;
    }
  }
}
</style>
