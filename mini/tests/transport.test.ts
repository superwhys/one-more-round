import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createRequire } from 'node:module'
import fs from 'node:fs'
import path from 'node:path'
import ts from 'typescript'
import vm from 'node:vm'

const root = path.resolve(import.meta.dirname, '../miniprogram')
// load executes a native module against a narrow WeChat fake without a browser.
function load(file: string, wx: object = {}, cache = new Map<string, { exports: unknown }>()) {
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

test('transport checks HTTP and business errors independently', () => {
  const api = load('utils/api')
  assert.equal(api.parseResponse(200, { code: 0, data: 0 }), 0)
  assert.throws(() => api.parseResponse(200, { code: 12, message: '冲突', data: null }), /冲突/)
  assert.throws(() => api.parseResponse(403, { code: 0, data: {} }), /请求失败/)
  assert.throws(() => api.parseResponse(200, { code: 0 }), /缺少数据/)
})
test('in-flight responses from an old session cannot reach the new account', async () => {
  let succeed: Function = () => {}
  const cache = new Map()
  const api = load(
    'utils/api',
    {
      request(options: { success: Function }) {
        succeed = options.success
      },
    },
    cache,
  )
  const credential = load('utils/credential', {}, cache)
  credential.setToken('old')
  const response = api.request('/me')
  credential.setToken('new')
  succeed({ statusCode: 200, data: { code: 0, data: { id: 'old-account' } } })
  await assert.rejects(response, /账号已切换/)
})
test('requests carry bearer tokens only in headers and preserve retry keys', async () => {
  let options: Record<string, any> = {}
  const cache = new Map()
  const api = load(
    'utils/api',
    {
      request(value: typeof options) {
        options = value
        value.success({ statusCode: 200, data: { code: 0, data: { id: 'round' } } })
      },
    },
    cache,
  )
  load('utils/credential', {}, cache).setToken('private-token')
  await api.send('/groups/g/rounds', { memory: 'hello' }, 'POST', 'retry-key')
  assert.equal(options.header.Authorization, 'Bearer private-token')
  assert.equal(options.header['Idempotency-Key'], 'retry-key')
  assert.ok(!options.url.includes('private-token'))
})
test('invitation parser supports current web fragments and mini links', () => {
  const ui = load('utils/ui')
  assert.equal(ui.invitationToken('https://example.com/join#raw%2Btoken'), 'raw+token')
  assert.equal(ui.invitationToken('/pages/login/index?group_token=abc'), 'abc')
  assert.equal(ui.invitationToken(' raw-token '), 'raw-token')
  assert.equal(ui.invitationToken('https://example.com/join#%broken'), '')
})

test('an old me request cannot revoke the freshly rotated session', async () => {
  let succeed: Function = () => {}
  const cache = new Map()
  const wx = {
    request(options: { success: Function }) {
      succeed = options.success
    },
    getStorageSync() {
      return ''
    },
    getStorageInfoSync() {
      return { keys: [] }
    },
    reLaunch() {
      throw new Error('stale request must not redirect')
    },
  }
  const session = load('utils/session', wx, cache)
  session.acceptLogin({ id: 'member', email: 'member@example.test', token: 'old' })
  const pending = session.requireSession()
  session.acceptLogin({ id: 'member', email: 'member@example.test', token: 'new' })
  succeed({ statusCode: 401, data: { code: 1, message: 'expired', data: null } })
  await assert.rejects(pending, /账号已切换/)
  assert.equal(load('utils/credential', wx, cache).getToken(), 'new')
  assert.equal(session.getUser().id, 'member')
})
