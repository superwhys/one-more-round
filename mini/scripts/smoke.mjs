import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import automator from 'miniprogram-automator'

// Run only with scripts/mini-fixture.sh; all mutations stay in its disposable DB.
const deadline = setTimeout(() => {
  console.error('Mini smoke timed out')
  process.exit(1)
}, 90000)
deadline.unref()
const origin = process.env.OMR_MINI_FIXTURE || 'http://127.0.0.1:8080'
const status = await (await fetch(`${origin}/api/v1/status`)).json()
assert.equal(status.data?.version, 'mini-ui-fixture', 'Refusing to run against a non-fixture server')
const mini = await automator.connect({ wsEndpoint: process.env.WECHAT_AUTO_WS || 'ws://127.0.0.1:9420' })
console.log('Connected to WeChat Developer Tools')
const exceptions = []
mini.on('exception', event => exceptions.push(event))
try {
  await mini.mockWxMethod('login', { code: 'mini-fixture-code', errMsg: 'login:ok' })
  await mini.mockWxMethod('showModal', { confirm: true, cancel: false, errMsg: 'showModal:ok' })
  let page = await mini.reLaunch('/pages/login/index')
  await page.waitFor(500)
  page = await mini.currentPage()
  if (page.path === 'pages/login/index') {
    assert.equal(await page.$('input[data-field="email"]'), null)
    await page.callMethod('toggle')
    await (await page.$('input[data-field="email"]')).input('mini-existing@example.test')
    await page.callMethod('sendCode')
    assert.equal(await page.data('error'), '')
    const { code } = await (await fetch(`${origin}/__test__/code?email=mini-existing@example.test`)).json()
    await (await page.$('input[data-field="emailCode"]')).input(code)
    await page.callMethod('login')
    await page.waitFor(300)
  }
  console.log('Checking timeline')
  page = await mini.switchTab('/pages/review/index')
  await page.waitFor(300)
  assert.equal(await page.data('error'), '')
  assert.equal((await page.data('group')).name, '开发者工具测试小组')
  await (await page.$('.record-button')).tap()
  await page.waitFor(500)
  page = await mini.currentPage()
  await page.waitFor(300)
  assert.equal(page.path, 'pages/round-form/index')
  assert.equal(await page.data('error'), '')
  await page.callMethod('discardDraft')
  await page.waitFor(100)
  await (await page.$('.game-option')).tap()
  const players = await page.data('players')
  for (const player of players)
    if (!player.selected)
      await (
        (await page.$(`button[data-id="${player.id}"][bindtap="togglePlayer"]`)) ||
        (await page.$(`button[data-id="${player.id}"]`))
      ).tap()
  // The first picker is the date; the second chooses explicit outcomes.
  const pickers = await page.$$('picker')
  await pickers[1].trigger('change', { value: '1' })
  await page.waitFor(200)
  const participants = await page.data('participants')
  await page.callMethod('toggleWinner', { currentTarget: { dataset: { id: participants[0].id } } })
  await page.waitFor(100)
  const scores = await page.$$('input[data-id]')
  if (scores[0]) await scores[0].input('-1.25')
  if (scores[1]) await scores[1].input('0')
  if (!(await page.data('more'))) await page.callMethod('toggleMore')
  await page.waitFor('textarea')
  await (await page.$('textarea')).input('开发者工具真实接口验证：又一局，记下相聚。')
  console.log('Checking photo upload')
  const png = await readFile(new URL('../miniprogram/pages/round/share-card.png', import.meta.url))
  const photoPath = await mini.evaluate(base64 => {
    const filePath = wx.env.USER_DATA_PATH + '/omr-smoke-photo.png'
    wx.getFileSystemManager().writeFileSync(filePath, base64, 'base64')
    return filePath
  }, png.toString('base64'))
  assert.equal(typeof photoPath, 'string')
  await mini.mockWxMethod('chooseMedia', {
    type: 'image',
    tempFiles: [{ tempFilePath: photoPath, size: png.length, fileType: 'image' }],
  })
  await page.callMethod('choosePhotos')
  assert.equal((await page.data('uploads')).length, 0, JSON.stringify(await page.data('uploads')))
  assert.equal((await page.data('form.photos')).length, 1)
  await page.callMethod('save')
  assert.equal(await page.data('error'), '')
  const id = await page.data('savedID')
  assert.ok(id)
  await ((await page.$('button[bindtap="viewSaved"]')) || (await page.$('.primary'))).tap()
  await page.waitFor(500)
  page = await mini.currentPage()
  await page.waitFor(300)
  assert.equal(page.path, 'pages/round/index')
  assert.equal(await page.data('error'), '')
  assert.match((await page.data('detail')).memory, /真实接口验证/)
  await (await page.$('textarea')).input('这一局的评论。')
  await page.callMethod('submitComment')
  assert.equal(await page.data('commentsError'), '')
  assert.equal((await page.data('comments')).length, 1)
  await mini.screenshot({ path: '/tmp/one-more-round-mini-detail.png' })
  page = await mini.switchTab('/pages/review/index')
  await page.waitFor(300)
  assert.ok((await page.data('total')) > 0)
  await mini.screenshot({ path: '/tmp/one-more-round-mini-review.png' })
  page = await mini.switchTab('/pages/games/index')
  await page.waitFor(300)
  assert.equal(await page.data('error'), '')
  assert.equal(await page.$('.modal-mask'), null)
  await mini.screenshot({ path: '/tmp/one-more-round-mini-games.png' })
  page = await mini.navigateTo('/pages/recap/index')
  await page.waitFor(300)
  assert.equal(await page.data('error'), '')
  await page.callMethod('generateCard')
  assert.equal(await page.data('saveError'), '')
  assert.ok(await page.data('cardPath'))
  // Exercise the two other modes through the native page's event handlers.
  for (const mode of ['coop', 'team']) {
    console.log('Checking mode', mode)
    page = await mini.navigateTo('/pages/round-form/index')
    await page.waitFor(300)
    await page.callMethod('discardDraft')
    await page.callMethod('chooseGame', { currentTarget: { dataset: { id: (await page.data('games'))[0].id } } })
    const roster = await page.data('players')
    const selected = mode === 'coop' ? roster.slice(0, 1) : roster.slice(0, 2)
    for (const player of selected)
      await page.callMethod('togglePlayer', { currentTarget: { dataset: { id: player.id } } })
    await page.callMethod('changeMode', { currentTarget: { dataset: { mode } } })
    await page.callMethod('changeOutcome', { detail: { value: '1' } })
    if (mode === 'team') {
      for (const [index, player] of selected.entries())
        await page.callMethod('assignTeam', {
          currentTarget: { dataset: { id: player.id } },
          detail: { value: String(index + 1) },
        })
      await page.callMethod('toggleTeamWinner', { currentTarget: { dataset: { index: 0 } } })
      await page.callMethod('inputTeam', {
        currentTarget: { dataset: { index: 0, field: 'score' } },
        detail: { value: '-2.5' },
      })
    } else
      await page.callMethod('inputField', {
        currentTarget: { dataset: { field: 'team_score' } },
        detail: { value: '0' },
      })
    await page.callMethod('save')
    assert.equal(await page.data('error'), '')
    const roundID = await page.data('savedID')
    assert.ok(roundID)
    await page.callMethod('viewSaved')
    await page.waitFor(400)
    page = await mini.currentPage()
    assert.equal(await page.data('error'), '')
    if (mode === 'team') {
      console.log('Checking edit/share/recycle')
      await page.callMethod('edit')
      await page.waitFor(400)
      page = await mini.currentPage()
      await page.callMethod('inputField', {
        currentTarget: { dataset: { field: 'memory' } },
        detail: { value: '组队对局编辑验证' },
      })
      await page.callMethod('save')
      await page.waitFor(400)
      page = await mini.currentPage()
      assert.equal((await page.data('detail')).memory, '组队对局编辑验证')
      await page.callMethod('openShare')
      await page.callMethod('generateShare')
      assert.equal(await page.data('shareError'), '')
      const shareToken = await page.data('shareToken')
      assert.ok(shareToken)
      page = await mini.navigateTo('/pages/round/index?token=' + shareToken)
      await page.waitFor(300)
      assert.equal(await page.data('public'), true)
      assert.equal(await page.$('.comments'), null)
      assert.equal((await page.data('detail')).author, '')
      page = await mini.navigateBack()
      await page.waitFor(300)
      await page.callMethod('openShare')
      await page.callMethod('revokeShare')
      assert.equal(await page.data('shareActive'), false)
      await page.callMethod('remove')
      await page.waitFor(400)
      page = await mini.navigateTo('/pages/recycle/index')
      await page.waitFor(300)
      assert.ok((await page.data('items')).some(round => round.id === roundID))
      await page.callMethod('restore', { currentTarget: { dataset: { id: roundID } } })
      assert.equal(await page.data('error'), '')
      assert.ok(!(await page.data('items')).some(round => round.id === roundID))
    }
    await mini.switchTab('/pages/review/index')
  }
  console.log('Checking album')
  page = await mini.navigateTo('/pages/album/index')
  await page.waitFor(500)
  assert.equal(await page.data('error'), '')
  assert.ok((await page.data('days')).length > 0)
  console.log(
    'Passed: solo coop, team assignment/results, edit, public projection/revocation, recycle restore and album',
  )
  assert.deepEqual(exceptions, [])
  console.log(
    'Passed: bind/login, real API round creation with decimal/zero, photo preprocessing/upload/download, detail, comments, timeline, games and recap card',
  )
} finally {
  await mini.restoreWxMethod('login')
  await mini.restoreWxMethod('showModal')
  await mini.restoreWxMethod('chooseMedia')
  mini.disconnect()
}
