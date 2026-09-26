// appShare sends a fixed home card without exposing the current page's private content.
export function appShare(): { title: string; path: string; imageUrl: string } {
  return {
    title: '又一局 · 记下每一局的输赢与相聚',
    path: '/pages/review/index',
    imageUrl: '/pages/round/share-card.png',
  }
}
