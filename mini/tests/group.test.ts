import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'

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
  const modules: Record<string, unknown> = {
    '../../utils/session': { requireGroup: async () => ({ user: { id: 'owner' }, group, snapshot }) },
    '../../utils/share': { appShare: () => ({}) },
    '../../utils/ui': { errorMessage: (error: Error) => error.message },
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
  })
  const page = {
    ...definition,
    data: structuredClone(definition.data),
    // setData applies the group's top-level view updates.
    setData(updates: Record<string, unknown>) {
      Object.assign(this.data, updates)
    },
  }
  await page.load()
  assert.equal(page.data.error, '')
  assert.deepEqual(JSON.parse(JSON.stringify(page.data.members)), [
    { user_id: 'owner', email: 'owner@example.test', playerName: '组主昵称' },
    { user_id: 'member', email: 'member@example.test', playerName: '成员昵称' },
    { user_id: 'wechat', email: '', playerName: '微信昵称' },
    { user_id: 'unlinked', email: '', playerName: '' },
  ])
  assert.deepEqual(snapshot.members.map(member => member.user_id), ['member', 'wechat', 'unlinked', 'owner'])
  group.owner = 'wechat'
  await page.load()
  assert.deepEqual(Array.from(page.data.members, (member: { user_id: string }) => member.user_id), [
    'wechat', 'member', 'unlinked', 'owner',
  ])
})
