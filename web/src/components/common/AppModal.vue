<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import Icon from './AppIcon.vue'
defineProps<{ title: string; wide?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement>()
let previousFocus: HTMLElement | null = null
onMounted(() => { previousFocus = document.activeElement as HTMLElement; dialog.value?.showModal() })
onBeforeUnmount(() => { dialog.value?.close(); previousFocus?.focus() })
</script>
<template>
  <dialog ref="dialog" class="d-modal" :class="{ wide }" aria-labelledby="design-dialog-title" @cancel.prevent="emit('close')" @click="(event) => { if (event.target === dialog) emit('close') }">
    <header><div><p class="d-eyebrow">ONE MORE ROUND</p><h2 id="design-dialog-title">{{ title }}</h2></div><button class="d-icon-button" aria-label="关闭弹窗" @click="emit('close')"><Icon name="close" /></button></header>
    <slot />
  </dialog>
</template>
