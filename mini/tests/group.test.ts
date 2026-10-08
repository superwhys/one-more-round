import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'

// groupPage loads the real group page with controlled membership and management calls.
function groupPage(snapshot: Record<string, any>) {
  const requests: { path: string; data: unknown }[] = []
  const modules: Record<string, unknown> = {
    '../../utils/api': {
      send: async (path: string, data: unknown) => { requests.push({ path, data: structuredClone(data) }) },
    },
    '../../utils/session': { requireGroup: async () => ({ user: { id: 'owner' }, group: snapshot.group, snapshot }) },
    '../../utils/share': { appShare: () => ({}) },
    '../../utils/ui': { errorMessage: (error: Error) => error.message, confirm: async () => true },
  }
  let definition: Record<string, any> = {}
  const code = ts.transpileModule(
    fs.readFileSync(new URL('../miniprogram/pages/group/index.ts', import.meta.url), 'utf8'),
    { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } },
  ).outputText
  vm.runInNewContext(code, {
    exports: {},
    require: (name: string) => modules[name] || {},
    Page: (options: Record<string, any>) => {
      definition = options
    },
    wx: { showToast() {} },
  })
  const page = {
    ...definition,
    data: structuredClone(definition.data),
    // setData applies the group's top-level view updates.
    setData(updates: Record<string, unknown>) {
      Object.assign(this.data, updates)
    },
  }
  return { page, requests }
}

test('group members retain accounts and linked players with the current owner first', async () => {
  const group = { id: 'group', name: '朋友小组', owner: 'owner' }
  const snapshot = {
    group,
    members: [
      { user_id: 'member', email: 'member@example.test' },
      { user_id: 'wechat', email: '' },
      { user_id: 'unlinked', email: '' },
      { user_id: 'owner', email: 'owner@example.test' },
    ],
    players: [
      { id: 'a', account: 'member', name: '成员昵称' },
      { id: 'b', account: 'wechat', name: '微信昵称' },
      { id: 'c', account: 'owner', name: '组主昵称' },
      { id: 'd', account: null, name: '未关联玩家' },
    ],
    games: [],
    claims: [],
  }
  const { page } = groupPage(snapshot)
  await page.load()
  assert.equal(page.data.error, '')
  assert.deepEqual(JSON.parse(JSON.stringify(page.data.members)), [
    { user_id: 'owner', email: 'owner@example.test', playerName: '组主昵称', name: 'owner@example.test' },
    { user_id: 'member', email: 'member@example.test', playerName: '成员昵称', name: 'member@example.test' },
    { user_id: 'wechat', email: '', playerName: '微信昵称', name: '微信昵称' },
    { user_id: 'unlinked', email: '', playerName: '', name: '小组成员' },
  ])
  assert.deepEqual(snapshot.members.map(member => member.user_id), ['member', 'wechat', 'unlinked', 'owner'])
  group.owner = 'wechat'
  await page.load()
  assert.deepEqual(Array.from(page.data.members, (member: { user_id: string }) => member.user_id), [
    'wechat', 'member', 'unlinked', 'owner',
  ])
})

test('duplicate display names remain separate accounts in member display, claims and management', async () => {
  const snapshot = {
    group: { id: 'group', name: '朋友小组', owner: 'owner' },
    members: [
      { user_id: 'first', email: '', username: '桌友' },
      { user_id: 'second', email: '', username: '桌友' },
      { user_id: 'unnamed', email: '', username: '' },
    ],
    players: [{ id: 'player', account: null, name: '以前的玩家' }],
    games: [],
    claims: [
      { user_id: 'first', player_id: 'player' },
      { user_id: 'second', player_id: 'player' },
      { user_id: 'unnamed', player_id: 'player' },
    ],
  }
  const { page, requests } = groupPage(snapshot)
  await page.load()
  assert.deepEqual(Array.from(page.data.members, (member: { user_id: string; name: string }) => [member.user_id, member.name]), [
    ['first', '桌友'],
    ['second', '桌友'],
    ['unnamed', '小组成员'],
  ])
  assert.deepEqual(Array.from(page.data.claims, (claim: { user_id: string; name: string }) => [claim.user_id, claim.name]), [
    ['first', '桌友'],
    ['second', '桌友'],
    ['unnamed', '小组成员'],
  ])
  for (const target of ['first', 'second']) {
    await page.manage({ currentTarget: { dataset: { action: 'remove', target } } })
    assert.equal(page.data.error, '')
  }
  assert.deepEqual(requests, [
    { path: '/groups/group/manage', data: { action: 'remove', target: 'first', value: '' } },
    { path: '/groups/group/manage', data: { action: 'remove', target: 'second', value: '' } },
  ])
})
