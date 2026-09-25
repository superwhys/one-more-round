// Keep the app session in memory; a cold start asks WeChat for a fresh login.
let token = ''
let generation = 0
// getToken reads the session credential from memory.
export function getToken(): string {
  return token
}
// getGeneration identifies the current credential lifetime for stale-response checks.
export function getGeneration(): number {
  return generation
}
// setToken rotates the memory credential and invalidates older requests.
export function setToken(value: string): void {
  token = value
  generation++
}
