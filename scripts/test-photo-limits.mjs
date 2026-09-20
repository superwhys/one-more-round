// Invoked by TestPhotoLimitsBrowser against the real disposable API and database.
import assert from 'node:assert/strict'
import { join } from 'node:path'
const { chromium } = await import(process.env.OMR_PLAYWRIGHT_MODULE || 'playwright')
const fixture = JSON.parse(process.env.OMR_PHOTO_BROWSER_FIXTURE)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const photo = Buffer.from(fixture.photo, 'base64')
const atLimit = Buffer.alloc(2 * 1024 * 1024)
photo.copy(atLimit)
const tooLarge = Buffer.concat([atLimit, Buffer.alloc(1)])
const file = (name, buffer = photo) => ({ name, mimeType: 'image/png', buffer })
const headers = () => ({ Origin: fixture.origin, 'Idempotency-Key': crypto.randomUUID() })
try {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } })
  await context.addCookies([{ name: 'omr_session', value: fixture.session, url: fixture.origin, httpOnly: true, sameSite: 'Strict' }])
  const page = await context.newPage()
  page.setDefaultTimeout(10000)
  const errors = []
  let uploads = 0
  let saves = 0
  page.on('pageerror', error => errors.push(error.message))
  page.on('request', request => {
    if (request.method() === 'POST' && new URL(request.url()).pathname.endsWith('/photos')) uploads++
    if (request.method() === 'POST' && new URL(request.url()).pathname.endsWith('/rounds')) saves++
  })
  await page.goto(`${fixture.origin}/rounds/new?game=${fixture.gameID}`)
  await page.getByRole('button', { name: '合作', exact: true }).click()
  await page.getByLabel('本局结果', { exact: false }).selectOption('unknown')
  await page.getByRole('button', { name: '＋ 添加回忆、照片和更多信息', exact: true }).click()
  const picker = page.getByLabel('照片（最多 3 张，每张不超过 2 MB）', { exact: true })
  await picker.setInputFiles(file('too-large.png', tooLarge))
  await page.getByRole('alert').filter({ hasText: '不超过 2 MB' }).waitFor()
  assert.equal(uploads, 0, 'oversized photo must be rejected before upload')

  // The first three requests can be pending together; the fourth still cannot start.
  await picker.setInputFiles([file('one.png'), file('two.png'), file('three.png'), file('four.png')])
  await page.getByRole('alert').filter({ hasText: '最多 3 张照片' }).waitFor()
  await page.waitForFunction(() => document.querySelectorAll('img[alt="本局照片"]').length === 3 && Array.from(document.querySelectorAll('img[alt="本局照片"]')).every(img => img.complete && img.naturalWidth > 0) && !document.querySelector('progress'))
  assert.equal(uploads, 3)
  for (const width of [390, 320, 1280]) {
    await page.setViewportSize({ width, height: 900 })
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, `overflow at ${width}px`)
    await picker.scrollIntoViewIfNeeded()
    if (process.env.OMR_BROWSER_ARTIFACTS) await page.screenshot({ path: join(process.env.OMR_BROWSER_ARTIFACTS, `photo-limits-${width}.png`), fullPage: true })
  }
  const createdResponse = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/rounds'))
  await page.getByRole('button', { name: '保存这局', exact: true }).click()
  const created = await (await createdResponse).json()
  assert.equal(created.code, 0)
  assert.equal(created.data.photos.length, 3)
  await page.getByRole('heading', { name: '这一局，记下了。' }).waitFor()
  console.log('PASS: oversize rejected locally; batch of four uploads only three; three-photo round saved')

  await page.goto(`${fixture.origin}/edit/${created.data.id}`)
  await picker.waitFor()
  await picker.setInputFiles(file('fourth.png'))
  await page.getByRole('alert').filter({ hasText: '最多 3 张照片' }).waitFor()
  assert.equal(uploads, 3, 'editing existing photos must count towards the limit')
  await page.locator('.j-photos').getByRole('button', { name: '移除', exact: true }).first().click()
  const uploadedResponse = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/photos'))
  await picker.setInputFiles(file('exactly-2MiB.png', atLimit))
  const uploaded = await (await uploadedResponse).json()
  assert.equal(uploaded.code, 0, 'exactly 2 MiB must be accepted')
  await page.waitForFunction(() => document.querySelectorAll('img[alt="本局照片"]').length === 3 && Array.from(document.querySelectorAll('img[alt="本局照片"]')).every(img => img.complete && img.naturalWidth > 0) && !document.querySelector('progress'))
  assert.equal(await page.getByRole('alert').count(), 0, 'successful reselection must clear the previous error')
  const editedResponse = page.waitForResponse(response => response.request().method() === 'PUT' && new URL(response.url()).pathname.endsWith(`/rounds/${created.data.id}`))
  await page.getByRole('button', { name: '保存这局', exact: true }).click()
  const edited = await (await editedResponse).json()
  assert.equal(edited.code, 0)
  assert.equal(edited.data.photos.length, 3)
  console.log('PASS: edit counts existing photos; remove and replace works; exact 2 MiB accepted')

  const api = `${fixture.origin}/api/v1/groups/${fixture.groupID}`
  const enrichedResponse = await context.request.put(`${api}/rounds/${edited.data.id}`, { headers: headers(), data: { ...edited.data, memory: '第一次打通的桌边回忆', location: '老地方' } })
  assert.equal(enrichedResponse.status(), 200)
  const enriched = await enrichedResponse.json()
  assert.equal(enriched.code, 0)

  // Search, monthly recap and the share-card download use the real persisted round.
  await page.goto(`${fixture.origin}/`)
  await page.getByRole('button', { name: /筛选/ }).click()
  await page.getByLabel('搜索回忆或地点', { exact: true }).fill('打通')
  const filterSelect = label => page.locator('#round-filters label').filter({ hasText: label }).locator('select')
  await filterSelect('地点').selectOption('老地方')
  await filterSelect('模式').selectOption('coop')
  await filterSelect('结果').selectOption('unknown')
  await filterSelect('照片').selectOption('true')
  await page.getByRole('button', { name: '应用筛选', exact: true }).click()
  await page.getByRole('heading', { name: fixture.gameName, exact: true }).waitFor()
  await page.getByRole('link', { name: /月度 \/ 年度回顾/ }).click()
  await page.getByRole('heading', { name: '这一段相聚。' }).waitFor()
  assert.equal(await page.locator('input[type="month"], input[type="number"]').count(), 0, 'recap must not use browser-native month or number controls')
  const yearControl = page.getByLabel('年份', { exact: false })
  const monthControl = page.getByLabel('月份', { exact: false })
  assert.equal(await yearControl.getAttribute('type'), 'text')
  assert.equal(await monthControl.evaluate(element => getComputedStyle(element).borderRadius), '9px')
  await page.getByText('1局').first().waitFor()
  const cardDownload = page.waitForEvent('download')
  await page.getByRole('button', { name: '下载分享卡片', exact: true }).click()
  assert.match((await cardDownload).suggestedFilename(), /又一局-.*\.png/)
  for (const width of [320, 1280]) {
    await page.setViewportSize({ width, height: 900 })
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, `recap overflow at ${width}px`)
  }
  const annualResponse = page.waitForResponse(response => new URL(response.url()).pathname.endsWith('/rounds/recap') && new URL(response.url()).searchParams.get('period') === new Date().getFullYear().toString())
  await page.getByRole('button', { name: '年度回顾', exact: true }).click()
  await annualResponse
  assert.equal(await page.getByLabel('月份', { exact: false }).count(), 0, 'annual recap only shows the year control')
  console.log('PASS: advanced search, monthly recap and local PNG share card work')

  // The owner can export a backup and restore a deleted round from the seven-day recycle bin.
  await page.goto(`${fixture.origin}/group`)
  const backupDownload = page.waitForEvent('download')
  await page.getByRole('button', { name: '导出小组备份', exact: true }).click()
  assert.match((await backupDownload).suggestedFilename(), /又一局-.*\.zip/)
  await page.goto(`${fixture.origin}/rounds/${edited.data.id}`)
  await page.getByRole('button', { name: '删除这局', exact: true }).click()
  await page.locator('.d-modal').getByRole('button', { name: '删除这局', exact: true }).click()
  await page.waitForURL(fixture.origin + '/')
  await page.goto(`${fixture.origin}/recycle-bin`)
  await page.getByText(fixture.gameName, { exact: true }).waitFor()
  await page.getByRole('button', { name: '恢复', exact: true }).click()
  await page.getByText('对局已经恢复到时间线').waitFor()
  await page.goto(`${fixture.origin}/`)
  await page.getByRole('heading', { name: fixture.gameName, exact: true }).waitFor()
  await page.getByRole('link', { name: '通知', exact: true }).click()
  await page.getByRole('heading', { name: '通知。' }).waitFor()
  await page.getByRole('heading', { name: '暂时没有新消息。' }).waitFor()
  console.log('PASS: ZIP export, recycle restore and notification center render correctly')

  const rejectedUpload = await context.request.post(`${api}/photos`, { headers: headers(), multipart: { photo: file('oversized.png', tooLarge) } })
  assert.equal(rejectedUpload.status(), 400)
  assert.match((await rejectedUpload.json()).message, /不超过 2 MB/)
  const fourPhotos = [...created.data.photos, uploaded.data.id]
  for (const method of ['post', 'put']) {
    const url = method === 'post' ? `${api}/rounds` : `${api}/rounds/${edited.data.id}`
    const response = await context.request[method](url, { headers: headers(), data: { ...edited.data, photos: fourPhotos } })
    assert.equal(response.status(), 400)
    assert.match((await response.json()).message, /最多 3 张/)
  }
  console.log('PASS: direct API rejects oversize uploads and four-photo create/edit requests')

  // Old drafts stay intact until the user explicitly removes excess photos.
  await page.evaluate(({ userID, groupID, round, photos }) => {
    localStorage.setItem(`omr:draft:${userID}:${groupID}:new`, JSON.stringify({ form: { ...round, photos }, key: crypto.randomUUID() }))
  }, { ...fixture, round: enriched.data, photos: fourPhotos })
  await page.goto(`${fixture.origin}/rounds/new`)
  await page.getByText('已恢复本账号在这个小组的草稿。', { exact: false }).waitFor()
  const previousSaves = saves
  await page.getByRole('button', { name: '保存这局', exact: true }).click()
  await page.getByRole('alert').filter({ hasText: '请先移除多余照片' }).waitFor()
  assert.equal(await page.getByAltText('本局照片', { exact: true }).count(), 4)
  assert.equal(saves, previousSaves, 'oversized draft must not submit')
  assert.deepEqual(errors, [])
  console.log('PASS: oversized old draft is preserved and requires explicit removal')
} finally {
  await browser.close()
}
