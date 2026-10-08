import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import vm from 'node:vm'
import ts from 'typescript'
import type { Page as RoundPage, Round, Snapshot } from '../miniprogram/utils/types.ts'

const source = path.resolve(import.meta.dirname, '../miniprogram')
const kinds = ['review', 'games', 'group'] as const
type Tab = (typeof kinds)[number]
const snapshot: Snapshot = {
  group: { id: 'group', name: '朋友小组', owner: 'user' },
  games: [{ id: 'game', name: '卡卡颂', original: 'Carcassonne', bgg_id: null }],
  players: [{ id: 'player', name: '甲', account: 'user' }],
  members: [{ user_id: 'user', email: 'user@example.test' }],
  claims: [],
}
const context = { user: { id: 'user', email: 'user@example.test' }, group: snapshot.group, snapshot }
const round: Round = {
  id: 'round',
  game_id: 'game',
  date: '2026-10-02',
  mode: 'coop',
  outcome: 'unknown',
  players: ['player'],
  winners: [],
  scores: {},
  teams: [],
  team_score: null,
  memory: '',
  minutes: null,
  photos: ['photo'],
  author: 'user',
  updated_by: 'user',
  updated_at: '',
  version: 1,
  deleted_at: null,
}
const rounds: RoundPage = {
  activity: { game: { count: 1, last_date: round.date } },
  items: [{ ...round, photos: [] }],
  total: 1,
  games: 1,
  players: 1,
  stats: [],
}

// deferred keeps each remote dependency pending until its test explicitly releases it.
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((success, failure) => {
    resolve = success
    reject = failure
  })
  return { promise, resolve, reject }
}

// settle advances promise continuations without depending on arbitrary network timing.
function settle() {
  return new Promise<void>(resolve => setImmediate(resolve))
}

// tabPage executes native page lifecycle code with controllable membership, records and photos.
function tabPage(kind: Tab) {
  let definition: Record<string, any> = {}
  let membershipIndex = 0
  const memberships = [deferred<typeof context | null>()]
  const records = deferred<RoundPage>()
  const photo = deferred<string>()
  const requests: string[] = []
  const modules: Record<string, unknown> = {
    '../../utils/api': {
      query: () => '?limit=20&offset=0',
      request: (requestPath: string) => {
        requests.push(requestPath)
        return records.promise
      },
    },
    '../../utils/session': { requireGroup: () => memberships[membershipIndex++].promise },
    '../../utils/photo': { getPhoto: () => photo.promise },
    '../../utils/share': { appShare: () => ({}) },
    '../../utils/ui': { errorMessage: (error: Error) => error.message },
  }
  const code = ts.transpileModule(fs.readFileSync(path.join(source, 'pages', kind, 'index.ts'), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText
  vm.runInNewContext(code, {
    exports: {},
    require: (name: string) => modules[name] || {},
    Page: (options: Record<string, any>) => {
      definition = options
    },
    wx: {},
  })
  const page = {
    ...definition,
    data: structuredClone(definition.data),
    // setData applies native dotted paths so background photo updates reach the rendered card.
    setData(updates: Record<string, unknown>) {
      for (const [field, value] of Object.entries(updates)) {
        const parts = field.replace(/\[(\d+)\]/g, '.$1').split('.')
        let target = this.data
        for (const part of parts.slice(0, -1)) target = target[part]
        target[parts[parts.length - 1]] = value
      }
    },
  }
  return { page, memberships, records, photo, requests }
}

for (const kind of kinds) {
  test(`${kind} starts with first-screen loading before its lifecycle runs`, () => {
    const { page } = tabPage(kind)
    assert.equal(page.data[kind === 'review' ? 'initialLoading' : 'loading'], true)
  })

  test(`${kind} keeps loading through pending membership and exits after authorized data arrives`, async () => {
    const { page, memberships, records, requests } = tabPage(kind)
    const pending = page.load()
    assert.equal(page.data.loading, true)
    assert.equal(page.data.snapshot, null)
    memberships[0].resolve(context)
    await settle()
    if (kind !== 'group') {
      assert.equal(requests.length, 1)
      assert.equal(page.data.loading, true)
      if (kind === 'review') assert.equal(page.data.initialLoading, true)
      else assert.equal(page.data.snapshot.group.id, 'group', 'the local shelf can render before activity arrives')
      records.resolve(rounds)
    }
    await pending
    assert.equal(page.data.loading, false)
    assert.equal(page.data.error, '')
    assert.equal(page.data.snapshot.group.id, 'group')
    if (kind === 'review') {
      assert.equal(page.data.initialLoading, false)
      assert.equal(page.data.items[0].gameName, '卡卡颂')
    } else if (kind === 'games') {
      assert.equal(page.data.filteredGames[0].count, 1)
      assert.equal(page.data.activityLoaded, true)
    } else assert.equal(page.data.members[0].playerName, '甲')
  })

  test(`${kind} exits first-screen loading and exposes membership failures`, async () => {
    const { page, memberships } = tabPage(kind)
    const pending = page.load()
    memberships[0].reject(new Error('网络连接失败'))
    await pending
    assert.equal(page.data.loading, false)
    assert.equal(page.data.error, '网络连接失败')
    assert.equal(page.data[kind === 'review' ? 'initialLoading' : 'loading'], false)
  })
}

for (const kind of ['review', 'games'] as const) {
  test(`${kind} exits first-screen loading when its records request fails`, async () => {
    const { page, memberships, records } = tabPage(kind)
    const pending = page.load()
    memberships[0].resolve(context)
    await settle()
    records.reject(new Error('记录暂时无法加载'))
    await pending
    assert.equal(page.data.loading, false)
    assert.equal(page.data.error, '记录暂时无法加载')
    if (kind === 'review') assert.equal(page.data.initialLoading, false)
    else assert.equal(page.data.filteredGames[0].name, '卡卡颂', 'activity failure retains the authorized local shelf')
  })
}

test('review makes first-screen cards visible while a photo is still downloading', async () => {
  const { page, memberships, records, photo } = tabPage('review')
  const pending = page.load()
  memberships[0].resolve(context)
  records.resolve({ ...rounds, items: [round] })
  await settle()
  try {
    assert.equal(page.data.initialLoading, false)
    assert.equal(page.data.items.length, 1)
    assert.equal(page.data.items[0].image, '')
    assert.equal(page.data.loading, true, 'record pagination remains guarded until existing photo work finishes')
  } finally {
    photo.resolve('local-photo')
    await pending
  }
  assert.equal(page.data.items[0].image, 'local-photo')
  assert.equal(page.data.loading, false)
})

for (const failure of [false, true]) {
  test(`hidden group ${failure ? 'failure' : 'completion'} cannot finish or overwrite a newer load`, async () => {
    const { page, memberships } = tabPage('group')
    const oldLoad = page.load()
    page.onHide()
    memberships.push(deferred<typeof context | null>())
    const newLoad = page.load()
    if (failure) memberships[0].reject(new Error('旧请求失败'))
    else memberships[0].resolve(context)
    await oldLoad
    assert.equal(page.data.loading, true, 'the current request must retain its skeleton')
    assert.equal(page.data.error, '', 'an obsolete error must not replace the current request')
    assert.equal(page.data.snapshot, null)
    memberships[1].resolve(context)
    await newLoad
    assert.equal(page.data.loading, false)
    assert.equal(page.data.snapshot.group.id, 'group')
  })
}

test('tab pages opt into WeChat static first-render caches', () => {
  for (const kind of kinds) {
    const configuration = JSON.parse(fs.readFileSync(path.join(source, 'pages', kind, 'index.json'), 'utf8'))
    assert.equal(configuration.initialRenderingCache, 'static', kind)
  }
})

test('native tab WXML renders initial skeletons, content, empty states and failures distinctly', async testContext => {
  const compiler =
    process.env.WECHAT_WCC ||
    '/Applications/wechatwebdevtools.app/Contents/Resources/app.asar.unpacked/node_modules/wcc-exec/wcc'
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'omr-tab-wxml-'))
  try {
    const output = path.join(temporary, 'templates.js')
    const result = spawnSync(
      compiler,
      ['-o', output, ...kinds.map(kind => `pages/${kind}/index.wxml`), 'templates/tab-skeleton.wxml'],
      { cwd: source, encoding: 'utf8', timeout: 15000 },
    )
    if ((result.error as NodeJS.ErrnoException | undefined)?.code === 'ENOENT') {
      testContext.skip('WeChat WXML compiler is not installed; set WECHAT_WCC to run native rendering checks')
      return
    }
    assert.ifError(result.error)
    assert.equal(result.status, 0, result.stderr || result.stdout)
    const warnings: string[] = []
    const runtime = vm.createContext({
      window: {},
      console: {
        log: (message: unknown) => warnings.push(String(message)),
        warn: (message: unknown) => warnings.push(String(message)),
      },
    })
    vm.runInContext(fs.readFileSync(output, 'utf8'), runtime, { timeout: 10000 })
    // render runs the actual WeChat template output, including shared template imports.
    const render = (kind: Tab, data: object) => JSON.stringify(runtime.$gwx(`pages/${kind}/index.wxml`)(data, {}))
    for (const kind of kinds) {
      const { page, memberships, records } = tabPage(kind)
      const initial = render(kind, page.data)
      assert.ok(initial.includes('skeleton'), `${kind}: the initial data must already render a placeholder`)
      assert.ok(!initial.includes('这里还没有回忆'))
      assert.ok(!initial.includes('给桌游架添上第一盒'))
      assert.ok(!initial.includes('添加昵称玩家'))
      const pending = page.load()
      memberships[0].resolve(context)
      records.resolve(rounds)
      await pending
      const ready = render(kind, page.data)
      assert.ok(!ready.includes('skeleton'), `${kind}: data replaces the placeholder`)
      assert.ok(ready.includes(kind === 'group' ? '添加昵称玩家' : '卡卡颂'))
      const empty = render(kind, {
        ...page.data,
        items: [],
        games: [],
        filteredGames: [],
        members: [],
        snapshot: { ...snapshot, players: [], members: [], games: [] },
        total: 0,
      })
      assert.ok(!empty.includes('skeleton'))
      if (kind === 'review') assert.ok(empty.includes('这里还没有回忆'))
      else if (kind === 'games') assert.ok(empty.includes('给桌游架添上第一盒'))
      else assert.ok(empty.includes('同桌的朋友 · 0'))
      const failed = render(kind, {
        ...page.data,
        snapshot: null,
        items: [],
        games: [],
        filteredGames: [],
        error: '网络连接失败',
      })
      assert.ok(failed.includes('网络连接失败'), `${kind}: failures retain a visible retry state`)
      assert.ok(!failed.includes('skeleton'))
      assert.ok(!failed.includes('这里还没有回忆'))
      assert.ok(!failed.includes('给桌游架添上第一盒'))
    }
    assert.deepEqual(warnings, [], 'native WXML must not report missing templates or rendering exceptions')
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true })
  }
})
