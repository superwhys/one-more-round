import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'

// loginPage loads the real page with controlled authentication and navigation.
function loginPage(restoreSession: () => Promise<object | null>) {
  const navigation: string[] = []
  const requests: unknown[] = []
  let invitation = ''
  let definition: Record<string, any> = {}
  const modules = {
    '../../utils/session': {
      restoreSession,
      getInvitation: () => invitation,
      setInvitation: (token: string) => { invitation = token },
    },
    '../../utils/api': {
      send: async (path: string, data: unknown) => {
        requests.push({ path, data })
        return { name: '朋友小组' }
      },
    },
    '../../utils/share': { appShare: () => ({}) },
    '../../utils/ui': { errorMessage: (error: Error) => error.message, invitationToken: (value: string) => value },
  }
  const code = ts.transpileModule(
    fs.readFileSync(new URL('../miniprogram/pages/login/index.ts', import.meta.url), 'utf8'),
    { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } },
  ).outputText
  vm.runInNewContext(code, {
    exports: {},
    require: (name: keyof typeof modules) => modules[name],
    Page: (options: Record<string, any>) => { definition = options },
    wx: {
      switchTab: ({ url }: { url: string }) => navigation.push(url),
      redirectTo: ({ url }: { url: string }) => navigation.push(url),
    },
  })
  const page = {
    ...definition,
    data: structuredClone(definition.data),
    // setData applies top-level login form state updates.
    setData(updates: object) { Object.assign(this.data, updates) },
  }
  return { page, navigation, requests, invitation: () => invitation }
}

test('returning members wait for restoration and reach the timeline without tapping login', async () => {
  let finish: (user: object) => void = () => {}
  const { page, navigation } = loginPage(() => new Promise(resolve => { finish = resolve }))
  page.onLoad({})
  const pending = page.onShow()
  assert.equal(page.data.restoring, true)
  assert.deepEqual(navigation, [])
  finish({ id: 'member' })
  await pending
  assert.deepEqual(navigation, ['/pages/review/index'])
})

test('restored invite visitors continue to join confirmation without racing a public preview', async () => {
  const { page, navigation, requests, invitation } = loginPage(async () => ({ id: 'member' }))
  page.onLoad({ group_token: 'pending-invite' })
  assert.equal(invitation(), 'pending-invite')
  assert.deepEqual(requests, [])
  await page.onShow()
  assert.deepEqual(navigation, ['/pages/setup/index'])
  assert.deepEqual(requests, [])
})

test('unregistered visitors see the invitation form after automatic login', async () => {
  const { page, navigation, requests } = loginPage(async () => null)
  page.onLoad({ group_token: 'pending-invite' })
  await page.onShow()
  assert.equal(page.data.restoring, false)
  assert.equal(page.data.busy, false)
  assert.equal(page.data.groupName, '朋友小组')
  assert.deepEqual(navigation, [])
  assert.equal(requests.length, 1)
})

test('restoration failures expose the login form with a retryable error', async () => {
  const { page, navigation } = loginPage(async () => { throw new Error('连接中断或超时，请重试') })
  page.onLoad({})
  await page.onShow()
  assert.equal(page.data.restoring, false)
  assert.equal(page.data.busy, false)
  assert.equal(page.data.error, '连接中断或超时，请重试')
  assert.deepEqual(navigation, [])
})

test('finishing restoration after leaving the login page does not change the current page', async () => {
  let finish: (user: object) => void = () => {}
  const { page, navigation } = loginPage(() => new Promise(resolve => { finish = resolve }))
  page.onLoad({})
  const pending = page.onShow()
  page.onHide()
  finish({ id: 'member' })
  await pending
  assert.deepEqual(navigation, [])
})
