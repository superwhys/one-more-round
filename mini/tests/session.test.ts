import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createRequire } from 'node:module'
import fs from 'node:fs'
import path from 'node:path'
import ts from 'typescript'
import vm from 'node:vm'

const root = path.resolve(import.meta.dirname, '../miniprogram')
type CallbackOptions = Record<string, any>

// load executes real session dependencies against a fresh WeChat runtime.
function load(file: string, wx: object, cache = new Map<string, { exports: unknown }>()) {
  const filename = path.join(root, file.endsWith('.ts') ? file : file + '.ts')
  if (cache.has(filename)) return cache.get(filename)!.exports as Record<string, Function>
  const module = { exports: {} }
  cache.set(filename, module)
  const code = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText
  const require = (name: string) =>
    name.startsWith('.')
      ? load(path.relative(root, path.resolve(path.dirname(filename), name)), wx, cache)
      : createRequire(import.meta.url)(name)
  vm.runInNewContext('(function(require,module,exports,wx){' + code + '\n})', { Error, Promise, Date, Set })(
    require,
    module,
    module.exports,
    wx,
  )
  return module.exports as Record<string, Function>
}

// runtime keeps storage across simulated cold starts while resetting memory.
function runtime(storage = new Map<string, unknown>()) {
  const requests: CallbackOptions[] = []
  const logins: CallbackOptions[] = []
  const navigation: string[] = []
  const removedPhotos: string[] = []
  const wx = {
    login(options: CallbackOptions) {
      logins.push(options)
    },
    request(options: CallbackOptions) {
      requests.push(options)
    },
    getStorageSync: (key: string) => storage.get(key),
    setStorageSync: (key: string, value: unknown) => storage.set(key, value),
    removeStorageSync: (key: string) => storage.delete(key),
    getStorageInfoSync: () => ({ keys: [...storage.keys()] }),
    reLaunch: (options: CallbackOptions) => navigation.push(options.url),
    downloadFile: (options: CallbackOptions) => options.success({ statusCode: 200, tempFilePath: '/tmp/private.jpg' }),
    getFileSystemManager: () => ({ unlink: (options: CallbackOptions) => removedPhotos.push(options.filePath) }),
  }
  const cache = new Map()
  return {
    session: load('utils/session', wx, cache),
    credential: load('utils/credential', wx, cache),
    photo: load('utils/photo', wx, cache),
    requests,
    logins,
    navigation,
    removedPhotos,
    storage,
  }
}

// flush lets a completed WeChat proof reach the real HTTP transport.
async function flush(): Promise<void> {
  await new Promise(resolve => setImmediate(resolve))
}

// respond finishes a queued HTTP request with the application's envelope.
function respond(options: CallbackOptions, data: unknown, statusCode = 200, code = 0): void {
  options.success({ statusCode, data: { code, data, message: code ? '服务拒绝请求' : '' } })
}

const member = { id: 'member', email: 'member@example.test', token: 'private-token' }

test('a protected cold-start entry restores WeChat login before validating the member', async () => {
  const app = runtime(new Map([['omr:group:member', 'remembered-group']]))
  const pending = app.session.requireSession()
  assert.equal(app.logins.length, 1)
  assert.deepEqual(app.navigation, [])
  app.logins[0].success({ code: 'fresh-proof' })
  await flush()
  assert.ok(app.requests[0].url.endsWith('/auth/wx-login'))
  assert.deepEqual(JSON.parse(app.requests[0].data), { code: 'fresh-proof' })
  respond(app.requests[0], member)
  await flush()
  assert.ok(app.requests[1].url.endsWith('/me'))
  assert.equal(app.requests[1].header.Authorization, 'Bearer private-token')
  respond(app.requests[1], { id: member.id, email: member.email })
  assert.equal((await pending).id, member.id)
  assert.equal(app.session.getGroupID(), 'remembered-group')
  assert.deepEqual(app.navigation, [])
  assert.ok(![...app.storage.values()].includes(member.token))
})

test('concurrent restoration shares one proof and preserves the pending invitation', async () => {
  const app = runtime()
  app.session.setInvitation('group-invitation')
  const first = app.session.restoreSession()
  const second = app.session.restoreSession()
  assert.equal(app.logins.length, 1)
  app.logins[0].success({ code: 'shared-proof' })
  await flush()
  assert.equal(app.requests.length, 1)
  assert.deepEqual(JSON.parse(app.requests[0].data), { code: 'shared-proof' })
  respond(app.requests[0], member)
  assert.equal((await first).id, member.id)
  assert.equal((await second).id, member.id)
  assert.equal(app.session.getInvitation(), 'group-invitation')
  assert.equal((await app.session.restoreSession()).id, member.id)
  assert.equal(app.logins.length, 1)
  assert.ok(![...app.storage.values()].includes(member.token))
})

test('a cold-start deep link keeps its explicit group ahead of the saved preference', async () => {
  const app = runtime(new Map([['omr:group:member', 'remembered-group']]))
  app.session.setGroupID('linked-group')
  const pending = app.session.restoreSession()
  app.logins[0].success({ code: 'fresh-proof' })
  await flush()
  respond(app.requests[0], member)
  assert.equal((await pending).id, member.id)
  assert.equal(app.session.getGroupID(), 'linked-group')
  assert.equal(app.storage.get('omr:group:member'), 'linked-group')
})

test('an existing session does not request another WeChat proof', async () => {
  const app = runtime()
  app.session.acceptLogin(member)
  assert.equal((await app.session.restoreSession()).id, member.id)
  assert.equal(app.logins.length, 0)
  assert.equal(app.requests.length, 0)
})

test('an unregistered WeChat account stays logged out without consuming an invitation', async () => {
  const app = runtime()
  app.session.setInvitation('pending-invitation')
  const pending = app.session.restoreSession()
  app.logins[0].success({ code: 'unregistered-proof' })
  await flush()
  assert.deepEqual(JSON.parse(app.requests[0].data), { code: 'unregistered-proof' })
  respond(app.requests[0], null, 403, 100403)
  assert.equal(await pending, null)
  assert.equal(app.session.getUser(), null)
  assert.equal(app.session.getInvitation(), 'pending-invitation')
  assert.equal(await app.session.restoreSession(), null)
  assert.equal(app.logins.length, 1)
  assert.equal(app.credential.getToken(), '')
})

test('a protected entry redirects only after restoration confirms that registration is needed', async () => {
  const app = runtime()
  const pending = app.session.requireSession()
  assert.deepEqual(app.navigation, [])
  app.logins[0].success({ code: 'unregistered-proof' })
  await flush()
  respond(app.requests[0], null, 403, 100403)
  assert.equal(await pending, null)
  assert.deepEqual(app.navigation, ['/pages/login/index'])
  assert.equal(app.requests.length, 1)
})

test('WeChat failures remain visible and do not trigger another automatic attempt', async () => {
  const app = runtime()
  const pending = app.session.restoreSession()
  const rejected = assert.rejects(pending, /微信登录失败/)
  app.logins[0].fail({ errMsg: 'login:fail' })
  await rejected
  await app.session.restoreSession().catch(() => null)
  assert.equal(app.logins.length, 1)
  assert.equal(app.requests.length, 0)
})

test('network failures remain visible without repeatedly exchanging WeChat proofs', async () => {
  const app = runtime()
  const pending = app.session.restoreSession()
  const rejected = assert.rejects(pending, /连接中断或超时/)
  app.logins[0].success({ code: 'fresh-proof' })
  await flush()
  app.requests[0].fail({ errMsg: 'request:fail' })
  await rejected
  await app.session.restoreSession().catch(() => null)
  assert.equal(app.logins.length, 1)
  assert.equal(app.requests.length, 1)
  assert.equal(app.session.getUser(), null)
})

for (const [status, code] of [[403, 999], [500, 100403]]) {
  test(`unexpected service error ${status}/${code} remains visible during restoration`, async () => {
    const app = runtime()
    const pending = app.session.restoreSession()
    const rejected = assert.rejects(pending, (error: any) => error.status === status && error.code === code)
    app.logins[0].success({ code: 'fresh-proof' })
    await flush()
    respond(app.requests[0], null, status, code)
    await rejected
    assert.equal(app.session.getUser(), null)
    assert.deepEqual(app.navigation, [])
  })
}

test('a cleared session cannot be revived by an old WeChat proof', async () => {
  const app = runtime()
  const pending = Promise.allSettled([app.session.restoreSession()])
  app.session.clearSession()
  app.logins[0].success({ code: 'stale-proof' })
  await flush()
  if (app.requests[0]) respond(app.requests[0], member)
  await pending
  assert.equal(app.session.getUser(), null)
  assert.equal(app.credential.getToken(), '')
})

test('manual login cannot be overwritten by an older WeChat proof', async () => {
  const app = runtime()
  const pending = Promise.allSettled([app.session.restoreSession()])
  app.session.acceptLogin({ id: 'new-member', email: 'new@example.test', token: 'new-token' })
  app.logins[0].success({ code: 'stale-proof' })
  await flush()
  if (app.requests[0]) respond(app.requests[0], member)
  await pending
  assert.equal(app.session.getUser().id, 'new-member')
  assert.equal(app.credential.getToken(), 'new-token')
})

test('manual login cannot be overwritten by an older login response', async () => {
  const app = runtime()
  const pending = Promise.allSettled([app.session.restoreSession()])
  app.logins[0].success({ code: 'old-proof' })
  await flush()
  app.session.acceptLogin({ id: 'new-member', email: 'new@example.test', token: 'new-token' })
  respond(app.requests[0], member)
  await pending
  assert.equal(app.session.getUser().id, 'new-member')
  assert.equal(app.credential.getToken(), 'new-token')
})

test('a cleared session cannot be revived by an older login response or another automatic attempt', async () => {
  const app = runtime()
  const pending = Promise.allSettled([app.session.restoreSession()])
  app.logins[0].success({ code: 'old-proof' })
  await flush()
  app.session.clearSession()
  respond(app.requests[0], member)
  await pending
  assert.equal(app.session.getUser(), null)
  assert.equal(app.credential.getToken(), '')
  await app.session.restoreSession().catch(() => null)
  assert.equal(app.logins.length, 1)
})

test('successful logout clears private data and suppresses restoration across cold starts', async () => {
  const app = runtime(new Map<string, unknown>([
    ['omr:draft:member:group', { memory: 'private draft' }],
    ['omr:group:member', 'remembered-group'],
  ]))
  app.session.acceptLogin(member)
  await app.photo.getPhoto('group', 'photo')
  const pending = app.session.logout()
  assert.ok(app.requests[0].url.endsWith('/auth/logout'))
  respond(app.requests[0], {})
  await pending
  assert.equal(app.session.getUser(), null)
  assert.equal(app.credential.getToken(), '')
  assert.equal(app.storage.has('omr:draft:member:group'), false)
  assert.equal(app.storage.get('omr:group:member'), 'remembered-group')
  assert.deepEqual(app.removedPhotos, ['/tmp/private.jpg'])
  assert.deepEqual(app.navigation, ['/pages/login/index'])
  assert.equal(app.storage.get('omr:logged-out'), true)
  assert.equal(await app.session.restoreSession(), null)
  assert.equal(app.logins.length, 0)

  const restarted = runtime(app.storage)
  assert.equal(await restarted.session.restoreSession(), null)
  assert.equal(restarted.logins.length, 0)
  restarted.session.acceptLogin(member)
  assert.equal(app.storage.has('omr:logged-out'), false)
  assert.equal((await restarted.session.restoreSession()).id, member.id)
  assert.ok(![...app.storage.values()].includes(member.token))
})

test('failed logout preserves the session and does not suppress future restoration', async () => {
  const app = runtime()
  app.session.acceptLogin(member)
  const pending = app.session.logout()
  const rejected = assert.rejects(pending, /服务拒绝请求/)
  respond(app.requests[0], null, 500, 100500)
  await rejected
  assert.equal(app.session.getUser().id, member.id)
  assert.equal(app.credential.getToken(), member.token)
  assert.equal(app.storage.has('omr:logged-out'), false)
  assert.deepEqual(app.navigation, [])
})
