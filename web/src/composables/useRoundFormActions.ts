import { ref } from 'vue'
import { addGame, importGame } from '@/api/game'
import { addPlayer } from '@/api/group'
import { uploadPhoto } from '@/api/photo'
import { useGroupContext } from './useGroupContext'
export function useRoundFormActions() {
  const { groupId, refresh, handleAccessError } = useGroupContext()
  const notice = ref('')
  async function addItem(kind: 'games' | 'players', name: string) {
    const item = await (kind === 'games' ? addGame(groupId, name) : addPlayer(groupId, name)).catch(async cause => {
      await handleAccessError(cause)
      throw cause
    })
    try {
      await refresh()
    } catch {
      notice.value = '已添加，选项刷新失败，请保存草稿后重新打开页面'
    }
    return item
  }
  async function importExternal(bggId: number, name: string) {
    const item = await importGame(groupId, bggId, name).catch(async cause => {
      await handleAccessError(cause)
      throw cause
    })
    try {
      await refresh()
    } catch {
      notice.value = '已添加，选项刷新失败，请保存草稿后重新打开页面'
    }
    return item
  }
  return {
    addItem,
    importExternal,
    upload: (file: File) =>
      uploadPhoto(groupId, file).catch(async cause => {
        await handleAccessError(cause)
        throw cause
      }),
    notice,
  }
}
