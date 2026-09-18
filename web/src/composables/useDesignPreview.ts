import { inject, provide, ref, onScopeDispose } from 'vue'
import type { InjectionKey } from 'vue'
function createPreview() {
  const modal = ref<'' | 'guide' | 'invite' | 'group' | 'welcome' | 'association'>('')
  const toast = ref(''); const newGroup = ref('')
  let toastTimer: ReturnType<typeof setTimeout> | undefined
  function notify(message: string) { toast.value = message; clearTimeout(toastTimer); toastTimer = setTimeout(() => { toast.value = '' }, 3500) }
  onScopeDispose(() => clearTimeout(toastTimer))
  return { modal, toast, newGroup, notify }
}
const previewKey: InjectionKey<ReturnType<typeof createPreview>> = Symbol('design-preview')
export function provideDesignPreview() { const preview = createPreview(); provide(previewKey, preview); return preview }
export function useDesignPreview() { const preview = inject(previewKey); if (!preview) throw new Error('产品稿页面必须位于产品稿布局内'); return preview }
