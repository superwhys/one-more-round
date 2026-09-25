// errorMessage normalizes errors for consistent, readable feedback.
export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '操作失败，请重试'
}
// toast shows a short native operation message.
export function toast(error: unknown): void {
  wx.showToast({ title: typeof error === 'string' ? error : errorMessage(error), icon: 'none' })
}
// confirm waits for an explicit choice on a consequential action.
export function confirm(title: string, content: string): Promise<boolean> {
  return new Promise(resolve =>
    wx.showModal({
      title,
      content,
      confirmColor: '#b65e38',
      success: res => resolve(res.confirm),
      fail: () => resolve(false),
    }),
  )
}
// navigate handles native tab pages and regular page stacks.
export function navigate(url: string): void {
  if (['/pages/review/index', '/pages/games/index', '/pages/group/index'].includes(url)) wx.switchTab({ url })
  else wx.navigateTo({ url })
}
// invitationToken accepts either a raw invitation or a web invitation link.
export function invitationToken(value: string): string {
  const input = value.trim()
  const fragment = input.match(/\/join#([^?#]+)/)
  if (fragment) {
    try {
      return decodeURIComponent(fragment[1])
    } catch {
      return ''
    }
  }
  const match = input.match(/(?:[?#&](?:join|group_token|invite|token)=)([^&#]+)/)
  if (!match) return input
  try {
    return decodeURIComponent(match[1])
  } catch {
    return ''
  }
}
