import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import * as forms from '../miniprogram/utils/round-form.ts'

// formPage executes the native page against deterministic account and transport ports.
function formPage() {
  let definition: Record<string, any> = {}
  const storage = new Map<string, unknown>()
  const requests: { path: string; body: unknown; method: string; key: string }[] = []
  let failure: Error | null = null
  let activeUser = 'user'
  let activeGroup = 'group'
  const snapshot = {
    group: { id: 'group', name: '小组', owner: 'user' },
    members: [],
    claims: [],
    games: [{ id: 'game', name: '桌游', original: '', bgg_id: null }],
    players: [
      { id: 'a', name: '甲', account: 'user' },
      { id: 'b', name: '乙', account: null },
    ],
  }
  const modules: Record<string, unknown> = {
    '../../utils/api': {
      request: async () => snapshot,
      send: async (path: string, body: unknown, method: string, key: string) => {
        requests.push({ path, body, method, key })
        if (failure) throw failure
        return { ...(body as object), id: 'saved', version: 1 }
      },
    },
    '../../utils/session': {
      getGroupID: () => activeGroup,
      getUser: () => ({ id: activeUser }),
      setGroupID: (id: string) => {
        activeGroup = id
      },
      requireGroup: async () => ({ user: { id: activeUser }, group: snapshot.group, snapshot }),
    },
    '../../utils/ui': { confirm: async () => true, errorMessage: (error: Error) => error.message },
    '../../utils/photo': {
      getPhoto: async () => 'local-image',
      uploadPhoto: async () => {
        throw new Error('上传失败')
      },
    },
    '../../utils/round-form': forms,
  }
  const code = ts.transpileModule(
    fs.readFileSync(new URL('../miniprogram/pages/round-form/index.ts', import.meta.url), 'utf8'),
    { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } },
  ).outputText
  const wx = {
    getStorageSync: (key: string) => storage.get(key),
    setStorageSync: (key: string, value: unknown) => storage.set(key, structuredClone(value)),
    removeStorageSync: (key: string) => storage.delete(key),
    redirectTo() {},
  }
  vm.runInNewContext('(function(require,exports,Page,wx){' + code + '\n})', { Error, Promise, Date, Set })(
    (name: string) => modules[name],
    {},
    (options: Record<string, any>) => {
      definition = options
    },
    wx,
  )
  const page = {
    ...definition,
    data: structuredClone(definition.data),
    // setData applies native dotted property paths used by the form.
    setData(updates: Record<string, unknown>) {
      for (const [path, value] of Object.entries(updates)) {
        const parts = path.replace(/\[(\d+)\]/g, '.$1').split('.')
        let target = this.data
        for (const part of parts.slice(0, -1)) target = target[part]
        target[parts[parts.length - 1]] = value
      }
    },
  }
  return {
    page,
    storage,
    requests,
    snapshot,
    fail: (error: Error | null) => {
      failure = error
    },
    account: (id: string) => {
      activeUser = id
    },
  }
}

test('native form restores drafts only from the active account and group', async () => {
  const { page, storage } = formPage()
  const round = {
    ...forms.emptyRound(),
    game_id: 'game',
    players: ['a', 'b'],
    outcome: 'unknown',
    memory: '当前账号草稿',
  }
  storage.set('omr:draft:someone-else:group:new', { form: { ...round, memory: '别人的草稿' }, key: 'foreign' })
  storage.set('omr:draft:user:group:new', { form: round, key: 'same-attempt', minutes: '' })
  await page.initialize()
  assert.equal(page.data.form.memory, '当前账号草稿')
  assert.equal(page.submissionKey, 'same-attempt')
  assert.equal(page.data.restored, true)
  assert.equal(page.draftKey, 'omr:draft:user:group:new')
})

test('failed saves retain form and idempotency key, then success clears the draft', async () => {
  const fixture = formPage()
  await fixture.page.initialize()
  const page = fixture.page
  page.data.form = {
    ...forms.emptyRound(),
    game_id: 'game',
    players: ['a', 'b'],
    outcome: 'unknown',
    memory: '保留这段回忆',
    photos: ['uploaded'],
  }
  page.data.uploads = [{ key: 'failed-photo', path: 'local', size: 10, state: 'failed', error: '上传失败' }]
  page.persistDraft()
  const key = page.submissionKey
  fixture.fail(new Error('连接中断'))
  await page.save()
  assert.equal(page.data.error, '连接中断')
  assert.equal(page.data.form.memory, '保留这段回忆')
  assert.ok(fixture.storage.has(page.draftKey))
  fixture.fail(null)
  await page.save()
  assert.equal(fixture.requests.length, 2)
  assert.equal(fixture.requests[0].key, key)
  assert.equal(fixture.requests[1].key, key)
  assert.deepEqual(JSON.parse(JSON.stringify(fixture.requests[1].body)).photos, ['uploaded'])
  assert.equal(page.data.savedID, 'saved')
  assert.ok(!fixture.storage.has(page.draftKey))
  page.again()
  assert.notEqual(page.submissionKey, key)
  assert.equal(page.data.form.outcome, '')
  assert.deepEqual(Array.from(page.data.form.photos), [])
})

test('native form preserves the observed edit version and exposes concurrency conflicts', async () => {
  const fixture = formPage()
  await fixture.page.initialize()
  const page = fixture.page
  page.editingID = 'round'
  page.data.editing = true
  page.data.form = {
    ...forms.emptyRound(),
    id: 'round',
    game_id: 'game',
    players: ['a', 'b'],
    outcome: 'draw',
    version: 8,
  }
  fixture.fail(Object.assign(new Error('这条记录刚刚被修改，请刷新后再试'), { status: 409 }))
  await page.save()
  assert.equal(fixture.requests[0].method, 'PUT')
  assert.equal((fixture.requests[0].body as { version: number }).version, 8)
  assert.equal(page.data.conflict, true)
  assert.equal(page.data.form.version, 8)
})

test('delayed work cannot persist a draft into another account session', async () => {
  const fixture = formPage()
  await fixture.page.initialize()
  fixture.storage.clear()
  fixture.account('another-user')
  fixture.page.persistDraft()
  assert.equal(fixture.storage.size, 0)
})
