import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import vm from 'node:vm'
import ts from 'typescript'
import * as history from '../miniprogram/pages/game/history.ts'
import type { Page as RoundPage, Round, Snapshot } from '../miniprogram/utils/types.ts'

const source = path.resolve(import.meta.dirname, '../miniprogram')
const snapshot: Snapshot = {
  group: { id: 'group', name: '小组', owner: 'user' },
  games: [{ id: 'game', name: '卡卡颂', original: 'Carcassonne', bgg_id: null }],
  players: [
    { id: 'a', name: '甲', account: null },
    { id: 'b', name: '乙', account: null },
  ],
  members: [],
  claims: [],
}
const round: Round = {
  id: 'round',
  game_id: 'game',
  date: '2026-09-25',
  mode: 'individual',
  outcome: 'win',
  players: ['a', 'b'],
  winners: ['a', 'b'],
  scores: { a: '-1', b: '0' },
  teams: [],
  team_score: null,
  memory: '',
  minutes: null,
  photos: [],
  author: 'user',
  updated_by: 'user',
  updated_at: '',
  version: 1,
  deleted_at: null,
}

test('catalog history preserves explicit outcomes and keeps rates scoped to mode and valid samples', () => {
  assert.equal(history.historyCards([round], snapshot)[0].result, '♔ 甲、乙共同获胜')
  assert.equal(history.historyCards([{ ...round, outcome: 'draw' }], snapshot)[0].result, '＝ 全局平局')
  assert.match(history.historyCards([{ ...round, outcome: 'unknown' }], snapshot)[0].result, /未记结果/)
  assert.equal(
    history.historyCards([{ ...round, mode: 'coop', outcome: 'loss' }], snapshot)[0].result,
    '♡ 全队失败，下次再来',
  )
  assert.equal(
    history.historyCards(
      [
        {
          ...round,
          mode: 'team',
          teams: [
            {
              id: 't1',
              name: '红队',
              players: ['a'],
              score: null,
              winner: true,
            },
            {
              id: 't2',
              name: '蓝队',
              players: ['b'],
              score: '100',
              winner: false,
            },
          ],
        },
      ],
      snapshot,
    )[0].result,
    '♔ 红队获胜',
  )
  const stats: RoundPage['stats'] = [
    {
      game: 'game',
      player: 'a',
      mode: 'individual',
      played: 4,
      wins: 1,
      samples: 2,
    },
    {
      game: 'game',
      player: 'b',
      mode: 'individual',
      played: 1,
      wins: 0,
      samples: 0,
    },
    { game: 'game', player: 'a', mode: 'coop', played: 2, wins: 2, samples: 2 },
  ]
  assert.equal(history.statistics(stats, snapshot, 'individual', 'a')[0].rate, '50%')
  assert.equal(history.statistics(stats, snapshot, 'individual', 'b')[0].rate, '暂无数据')
  assert.equal(history.statistics(stats, snapshot, 'coop', 'a')[0].result, '2 成功 / 2 局')
  assert.deepEqual(history.filterSelection('player', 1, snapshot), {
    value: 'a',
    label: '甲',
  })
  assert.deepEqual(history.filterSelection('mode', 3, snapshot), {
    value: 'coop',
    label: '合作',
  })
  assert.equal(history.filterSelection('other', 0, snapshot), null)
  assert.notEqual(history.emptyFilters(), history.emptyFilters())
  assert.notEqual(history.emptyHistory().items, history.emptyHistory().items)
})

// catalogPage executes either detail page against deterministic membership and pagination ports.
function catalogPage(kind: 'game' | 'player') {
  let definition: Record<string, any> = {}
  const requests: URL[] = []
  const modules: Record<string, unknown> = {
    '../../utils/api': {
      ApiError: class ApiError extends Error {},
      query: (values: Record<string, string | number>) =>
        '?' +
        new URLSearchParams(
          Object.entries(values)
            .filter(([, value]) => value !== '')
            .map(([key, value]) => [key, String(value)]),
        ),
      request: async (requestPath: string): Promise<RoundPage> => {
        const url = new URL(requestPath, 'https://example.test')
        requests.push(url)
        const offset = Number(url.searchParams.get('offset'))
        return {
          activity: { game: { count: 2, last_date: round.date } },
          items: [{ ...round, id: `round-${offset}` }],
          total: 2,
          games: 1,
          players: 2,
          stats: [
            {
              game: 'game',
              player: 'a',
              mode: 'individual',
              played: 2,
              wins: 1,
              samples: 2,
            },
          ],
        }
      },
    },
    '../../utils/session': {
      requireGroup: async () => ({
        user: { id: 'user' },
        group: snapshot.group,
        snapshot,
      }),
      setGroupID() {},
    },
    '../../utils/photo': { getPhoto: async () => 'local-image' },
    '../../utils/ui': {
      errorMessage: (error: Error) => error.message,
      navigate() {},
      toast() {},
    },
    './history': history,
    '../game/history': history,
  }
  const code = ts.transpileModule(fs.readFileSync(path.join(source, 'pages', kind, 'index.ts'), 'utf8'), {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2020,
    },
  }).outputText
  vm.runInNewContext('(function(require,exports,Page,wx){' + code + '\n})', {
    Error,
    Promise,
    Date,
    Set,
  })(
    (name: string) => modules[name],
    {},
    (options: Record<string, any>) => {
      definition = options
    },
    {},
  )
  const page = {
    ...definition,
    data: structuredClone(definition.data),
    // setData supports the same dotted property paths as the native page runtime.
    setData(updates: Record<string, unknown>) {
      for (const [field, value] of Object.entries(updates)) {
        const parts = field.replace(/\[(\d+)\]/g, '.$1').split('.')
        let target = this.data
        for (const part of parts.slice(0, -1)) target = target[part]
        target[parts[parts.length - 1]] = value
      }
    },
  }
  page.data.id = kind === 'game' ? 'game' : 'a'
  return { page, requests }
}

test('native game and player pages retain fixed scope and applied filters across pagination', async () => {
  for (const kind of ['game', 'player'] as const) {
    const { page, requests } = catalogPage(kind)
    await page.load()
    page.filterInput({
      currentTarget: { dataset: { field: 'q' } },
      detail: { value: '尚未应用的条件' },
    })
    await page.load(true)
    const request = requests[requests.length - 1]
    assert.equal(request.pathname, '/groups/group/rounds')
    assert.equal(request.searchParams.get(kind), kind === 'game' ? 'game' : 'a')
    assert.equal(request.searchParams.get('q'), null, `${kind}: draft filter must not change pagination`)
    assert.equal(request.searchParams.get('offset'), '1')
    assert.deepEqual(
      Array.from(page.data.cards, (card: Round) => card.id),
      ['round-0', 'round-1'],
    )
    assert.equal(page.data.stats[0].rate, '50%')
    assert.equal(page.data.page.total, 2)
    // Await the load started by the native Apply handler without changing its production signature.
    const originalLoad = page.load.bind(page)
    let applied: Promise<void> = Promise.resolve()
    page.load = (...args: unknown[]) => {
      applied = originalLoad(...args)
      return applied
    }
    page.applyFilters()
    await applied
    const filtered = requests[requests.length - 1]
    assert.equal(filtered.searchParams.get('q'), '尚未应用的条件')
    assert.equal(filtered.searchParams.get('offset'), '0')
    assert.equal(page.data.cards.length, 1, `${kind}: applying a filter replaces the old list`)
  }
})

test('native WXML renders catalog modals, errors and shared detail templates conditionally', context => {
  const compiler =
    process.env.WECHAT_WCC ||
    '/Applications/wechatwebdevtools.app/Contents/Resources/app.asar.unpacked/node_modules/wcc-exec/wcc'
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'omr-catalog-wxml-'))
  try {
    const output = path.join(temporary, 'templates.js')
    const result = spawnSync(
      compiler,
      [
        '-o',
        output,
        'pages/games/index.wxml',
        'pages/game/index.wxml',
        'pages/game/history.wxml',
        'pages/wishlist/index.wxml',
        'pages/player/index.wxml',
      ],
      { cwd: source, encoding: 'utf8', timeout: 15000 },
    )
    if ((result.error as NodeJS.ErrnoException | undefined)?.code === 'ENOENT') {
      context.skip('WeChat WXML compiler is not installed; set WECHAT_WCC to run native rendering checks')
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
    vm.runInContext(fs.readFileSync(output, 'utf8'), runtime, {
      timeout: 10000,
    })
    // render executes WeChat's compiled templates instead of approximating WXML with an HTML parser.
    const render = (page: string, data: object) => JSON.stringify(runtime.$gwx(`pages/${page}/index.wxml`)(data, {}))
    const game = {
      ...snapshot.games[0],
      cover: '',
      letter: '卡',
      tone: 'sage',
      count: 7,
      lastDate: round.date,
    }
    const shelf = {
      games: [game],
      filteredGames: [game],
      search: '',
      loading: false,
      error: '',
      addOpen: false,
      externalOpen: false,
      activityLoaded: true,
    }
    assert.ok(!render('games', shelf).includes('modal-mask'))
    assert.ok(render('games', shelf).includes('7 局相聚'))
    assert.ok(render('games', { ...shelf, addOpen: true, name: '新桌游' }).includes('直接用此名称添加'))
    assert.ok(!render('games', { ...shelf, addOpen: true }).includes('搜索 BoardGameGeek'))
    assert.ok(
      render('games', {
        ...shelf,
        externalOpen: true,
        externalItems: [],
        externalSearched: true,
      }).includes('没有找到结果'),
    )
    assert.ok(
      !render('wishlist', {
        loading: false,
        error: '网络失败',
        snapshot: null,
        wishedGames: [],
      }).includes('下次玩什么？'),
    )
    const detail = {
      current: game,
      page: { total: 7, games: 1, players: 3 },
      cards: [],
      stats: [],
      modes: [],
      loading: false,
      error: '',
      filters: {},
      filterOpen: false,
      hasFilters: false,
      fixedGame: true,
      fixedPlayer: false,
    }
    assert.ok(render('game', detail).includes('暂无封面'))
    assert.ok(render('game', detail).includes('每一款，都有自己的故事'))
    assert.ok(!render('game', detail).includes('这款桌游不存在或已合并'))
    assert.ok(render('game', { ...detail, current: null }).includes('这款桌游不存在或已合并'))
    assert.ok(!render('game', { ...detail, current: null }).includes('每一款，都有自己的故事'))
    assert.ok(
      render('player', {
        ...detail,
        current: snapshot.players[0],
        commonGames: [{ ...game, count: 7 }],
      }).includes('小组昵称玩家'),
    )
    assert.deepEqual(warnings, [], 'native WXML must not report missing templates or rendering exceptions')
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true })
  }
})
