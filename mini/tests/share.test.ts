import assert from 'node:assert/strict'
import { test } from 'node:test'
import fs from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { appShare } from '../miniprogram/utils/share.ts'

// pageDefinition loads a native page without starting WeChat or making network requests.
function pageDefinition(name: string) {
  const source = fs.readFileSync(new URL(`../miniprogram/pages/${name}/index.ts`, import.meta.url), 'utf8')
  const code = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText
  let definition: Record<string, any> = {}
  vm.runInNewContext('(function(require,exports,Page){' + code + '\n})', { Date, Promise })(
    (path: string) => (path === '../../utils/share' ? { appShare } : {}),
    {},
    (options: Record<string, any>) => {
      definition = options
    },
  )
  return definition
}

test('every page offers the fixed home card without a private page screenshot', () => {
  const config = JSON.parse(fs.readFileSync(new URL('../miniprogram/app.json', import.meta.url), 'utf8'))
  const card = appShare()
  assert.equal(card.path, '/pages/review/index')
  assert.equal(card.imageUrl, '/pages/round/share-card.png')
  assert.ok(fs.existsSync(new URL('../miniprogram' + card.imageUrl, import.meta.url)))
  for (const route of config.pages) {
    const source = fs.readFileSync(new URL(`../miniprogram/${route}.ts`, import.meta.url), 'utf8')
    assert.match(source, /onShareAppMessage/, route)
    assert.doesNotMatch(source, /wx\.hideShareMenu\(/, route)
  }
})

test('round sharing uses only an explicitly available public token', () => {
  const page = pageDefinition('round')
  const detail = { game: '桌游', date: '2026-09-25' }
  const home = page.onShareAppMessage.call({ data: { shareToken: '', detail } })
  assert.equal(home.path, '/pages/review/index')
  const shared = page.onShareAppMessage.call({ data: { shareToken: 'token+/=', detail } })
  assert.equal(shared.path, '/pages/round/index?token=token%2B%2F%3D')
  assert.match(shared.title, /桌游/)
})

test('the invitation button keeps its explicit invite while the menu shares home', () => {
  const page = pageDefinition('invites')
  const context = { data: { token: 'invite+/=', name: '朋友小组' } }
  const menu = page.onShareAppMessage.call(context, { from: 'menu' })
  assert.equal(menu.path, '/pages/review/index')
  const button = page.onShareAppMessage.call(context, { from: 'button' })
  assert.equal(button.path, '/pages/login/index?group_token=invite%2B%2F%3D')
  context.data.token = ''
  assert.equal(page.onShareAppMessage.call(context, { from: 'button' }).path, '/pages/review/index')
})
