export function clearDrafts() {
  for (const key of Object.keys(localStorage)) {
    if (key.startsWith('omr:draft:')) localStorage.removeItem(key)
  }
}
