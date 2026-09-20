// Invoked by TestRoundShareBrowser against the real disposable API/database.
import assert from 'node:assert/strict'
import { join } from 'node:path'
const { chromium } = await import(process.env.OMR_PLAYWRIGHT_MODULE || 'playwright')
const fixture = JSON.parse(process.env.OMR_SHARE_BROWSER_FIXTURE)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
async function noOverflow(page, width) {
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, `page overflows at ${width}px`)
}
async function screenshot(page, name) {
  if (process.env.OMR_BROWSER_ARTIFACTS) await page.screenshot({ path: join(process.env.OMR_BROWSER_ARTIFACTS, `${name}.png`), fullPage: true })
}
try {
  const member = await browser.newContext({ viewport: { width: 390, height: 844 } })
  await member.addCookies([{ name: 'omr_session', value: fixture.session, url: fixture.origin, httpOnly: true, sameSite: 'Strict' }])
  const detail = await member.newPage()
  detail.on('pageerror', error => errors.push(error.message))
  await detail.goto(`${fixture.origin}/rounds/${fixture.roundID}`)
  await detail.getByRole('heading', { name: fixture.gameName, exact: true }).waitFor()
  await detail.getByRole('button', { name: '分享这局', exact: true }).click()
  await detail.getByRole('heading', { name: '把这一局分享出去', exact: true }).waitFor()
  await detail.getByRole('button', { name: '生成分享链接', exact: true }).click()
  const link = await detail.getByLabel('分享链接', { exact: true }).inputValue()
  assert.match(link, new RegExp(`^${fixture.origin.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/share#[0-9a-f]{64}$`))
  await noOverflow(detail, 390)
  await screenshot(detail, 'round-share-dialog-390')

  const anonymous = await browser.newContext({ viewport: { width: 390, height: 844 } })
  const shared = await anonymous.newPage()
  shared.on('pageerror', error => errors.push(error.message))
  await shared.goto(link)
  await shared.getByRole('heading', { name: fixture.gameName, exact: true }).waitFor()
  await shared.getByText(`${fixture.groupName} 分享的桌边回忆`, { exact: true }).waitFor()
  await shared.getByText(fixture.playerName, { exact: true }).waitFor()
  await shared.getByText(fixture.memory, { exact: true }).waitFor()
  await shared.waitForFunction(() => Array.from(document.querySelectorAll('img[alt="分享的对局照片"]')).every(image => image.complete && image.naturalWidth > 0))
  assert.equal(await shared.getByText('记录人：', { exact: false }).count(), 0)
  assert.equal((await anonymous.cookies()).some(cookie => cookie.name === 'omr_session'), false)
  for (const width of [390, 320, 1280]) {
    await shared.setViewportSize({ width, height: 900 })
    await noOverflow(shared, width)
    await screenshot(shared, `round-share-public-${width}`)
  }

  await detail.getByRole('button', { name: '撤销公开链接', exact: true }).click()
  await detail.getByRole('button', { name: '生成分享链接', exact: true }).waitFor()
  await shared.reload()
  await shared.getByRole('alert').filter({ hasText: '内容不存在' }).waitFor()
  assert.deepEqual(errors, [])
  await anonymous.close()
  await member.close()
  console.log('PASS: public round share generates, renders without a session at 320/390/1280px, loads photos and revokes immediately')
} finally {
  await browser.close()
}
