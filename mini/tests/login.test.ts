import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'

// loginPage loads the real page with controlled authentication and navigation.
function loginPage(restoreSession: () => Promise<object | null>, options: { groupID?: string; previewError?: string } = {}) {
  const navigation: string[] = []
  const requests: unknown[] = []
  const logins: unknown[] = []
  let invitation = ''
  let definition: Record<string, any> = {}
  const modules = {
    '../../utils/session': {
      restoreSession,
      getInvitation: () => invitation,
      setInvitation: (token: string) => { invitation = token },
      wxCode: async () => 'wechat-code',
      acceptLogin: (result: unknown) => { logins.push(result); invitation = '' },
    },
    '../../utils/api': {
      send: async (path: string, data: unknown) => {
        requests.push({ path, data: structuredClone(data) })
        if (path === '/auth/group-invite' && options.previewError) throw new Error(options.previewError)
        if (path === '/auth/wx-login') return { id: 'member', group_id: options.groupID }
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
    setInterval: () => 1,
    clearInterval: () => {},
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
  return { page, navigation, requests, logins, invitation: () => invitation }
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

for (const options of [{}, { invite: 'expired-trial', group_token: 'expired-group' }]) {
  test(`existing email binding only submits email proofs with ${Object.keys(options).length ? 'stale' : 'no'} invitations`, async () => {
    const { page, requests, navigation, logins, invitation } = loginPage(async () => null)
    page.onLoad(options)
    page.toggle()
    page.setData({ email: ' member@example.test ', emailCode: ' 123456 ' })

    await page.sendCode()
    assert.equal(page.data.error, '')
    assert.equal(page.data.countdown, 60)
    assert.deepEqual(requests, [{ path: '/auth/code', data: { email: 'member@example.test' } }])

    await page.login()
    assert.equal(page.data.error, '')
    assert.deepEqual(requests[1], {
      path: '/auth/wx-login',
      data: { code: 'wechat-code', email: 'member@example.test', email_code: '123456' },
    })
    assert.equal(logins.length, 1)
    assert.deepEqual(navigation, [options.group_token ? '/pages/setup/index' : '/pages/review/index'])
    assert.equal(invitation(), options.group_token || '')
  })
}

for (const options of [{ invite: 'trial-invite' }, { group_token: 'group-invite' }]) {
  test(`returning to WeChat registration keeps the ${options.invite ? 'trial' : 'group'} invitation`, async () => {
    const { page, requests } = loginPage(async () => null)
    page.onLoad(options)
    page.toggle()
    page.setData({ email: 'member@example.test', emailCode: '123456' })
    page.toggle()

    await page.login()
    assert.equal(page.data.error, '')
    assert.deepEqual(requests, [{
      path: '/auth/wx-login',
      data: { code: 'wechat-code', invite: options.invite || '', group_token: options.group_token || '' },
    }])
  })
}

test('returning from the mailbox in existing-account mode does not preview a stale invitation', async () => {
  const { page, requests } = loginPage(async () => null)
  page.onLoad({ group_token: 'expired-group' })
  page.toggle()
  page.onHide()

  await page.onShow()
  assert.deepEqual(requests, [])
  assert.equal(page.data.existing, true)
  assert.equal(page.data.busy, false)
  assert.equal(page.data.error, '')
})

test('an unavailable invitation does not prevent login and remains pending for explicit joining', async () => {
  const { page, requests, navigation, invitation } = loginPage(async () => null, { previewError: '邀请已失效' })
  page.onLoad({ group_token: 'revoked-group' })
  await page.onShow()
  assert.equal(page.data.error, '邀请已失效')

  await page.login()
  assert.equal(page.data.error, '')
  assert.equal(invitation(), 'revoked-group')
  assert.deepEqual(navigation, ['/pages/setup/index'])
  assert.deepEqual(requests[1], {
    path: '/auth/wx-login',
    data: { code: 'wechat-code', invite: '', group_token: 'revoked-group' },
  })
})

test('new accounts already joined by registration do not repeat the invitation flow', async () => {
  const { page, navigation, invitation } = loginPage(async () => null, { groupID: 'new-group' })
  page.onLoad({ group_token: 'group-invite' })
  await page.login()
  assert.equal(invitation(), '')
  assert.deepEqual(navigation, ['/pages/review/index'])
})
