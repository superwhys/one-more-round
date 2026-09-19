// Invoked by TestGroupInvitationBrowser against its disposable real API/database.
import assert from 'node:assert/strict'
import { join } from 'node:path'
const { chromium } = await import(process.env.OMR_PLAYWRIGHT_MODULE || 'playwright')
const fixture = JSON.parse(process.env.OMR_BROWSER_FIXTURE)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
async function visible(page, text) { await page.getByText(text, { exact: false }).first().waitFor({ state: 'visible' }) }
async function noOverflow(page) {
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, 'page overflows horizontally')
}
async function screenshot(page, name) {
  if (process.env.OMR_BROWSER_ARTIFACTS) await page.screenshot({ path: join(process.env.OMR_BROWSER_ARTIFACTS, `${name}.png`), fullPage: true })
}
try {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } })
  const page = await context.newPage()
  page.setDefaultTimeout(10000)
  page.on('pageerror', error => errors.push(error.message))
  await page.goto(`${fixture.origin}/join#${fixture.token}`)
  await visible(page, `加入「${fixture.name}」`)
  assert.equal(new URL(page.url()).pathname, '/login')
  assert.equal(new URL(page.url()).hash, '')
  assert.equal(await page.getByLabel('试用邀请码', { exact: false }).count(), 0)
  await noOverflow(page)
  await screenshot(page, 'invite-login-390')
  await page.setViewportSize({ width: 1280, height: 900 })
  await screenshot(page, 'invite-login-desktop')
  await page.setViewportSize({ width: 390, height: 844 })
  await page.reload()
  await visible(page, `加入「${fixture.name}」`)
  await page.getByLabel('邮箱', { exact: true }).fill('browser-friend@example.com')
  await page.getByRole('button', { name: '发送验证码', exact: true }).click()
  await page.getByLabel('邮箱验证码', { exact: false }).waitFor()
  await page.reload()
  await page.getByLabel('邮箱验证码', { exact: false }).waitFor()
  assert.equal(await page.getByLabel('邮箱', { exact: true }).inputValue(), 'browser-friend@example.com')
  await noOverflow(page)
  await screenshot(page, 'invite-code-390')
  const inbox = await (await context.request.get(`${fixture.origin}/__test__/code?email=browser-friend%40example.com`)).json()
  await page.getByLabel('邮箱验证码', { exact: false }).fill(inbox.code)
  await page.getByRole('button', { name: '登录并加入小组', exact: true }).click()
  await page.waitForURL('**/group')
  await visible(page, '你的玩家昵称')
  assert.equal(await page.evaluate(() => sessionStorage.getItem('omr:pending-invitation')), null)
  assert.equal(await page.evaluate(() => sessionStorage.getItem('omr:pending-login')), null)
  console.log('PASS: invitation signup and joining survive reload before and after code delivery')

  await page.goto(`${fixture.origin}/join#${fixture.token}`)
  await page.getByRole('button', { name: '进入小组', exact: true }).waitFor()
  await page.setViewportSize({ width: 1280, height: 900 })
  await noOverflow(page)
  await screenshot(page, 'invite-existing-member')
  await page.getByRole('button', { name: '进入小组', exact: true }).click()
  await page.waitForURL('**/group')
  console.log('PASS: existing member can enter without a duplicate join')

  await page.goto(`${fixture.origin}/join#${fixture.second}`)
  await page.getByRole('heading', { name: fixture.secondName }).waitFor()
  await page.getByRole('button', { name: '加入小组', exact: true }).click()
  await page.waitForURL('**/group')
  await page.getByRole('heading', { name: fixture.secondName, exact: false }).waitFor()
  console.log('PASS: signed-in account joins and switches to a second group')

  await page.goto(`${fixture.origin}/join#${fixture.token}`)
  await page.getByRole('button', { name: '进入小组', exact: true }).waitFor()
  await page.getByRole('button', { name: '退出登录', exact: true }).click()
  await page.waitForURL('**/login')
  await page.reload()
  await visible(page, '试用邀请码')
  assert.equal(await page.evaluate(() => sessionStorage.getItem('omr:pending-invitation')), null)
  console.log('PASS: logout clears pending invitations')

  await page.setViewportSize({ width: 320, height: 740 })
  for (const [token, message] of [[fixture.revoked, '小组邀请已撤销'], [fixture.expired, '小组邀请已过期'], ['unknown-token', '小组邀请无效']]) {
    await page.goto(`${fixture.origin}/join#${token}`)
    await visible(page, message)
    assert.equal(await page.getByRole('button', { name: '发送验证码', exact: true }).count(), 0)
    await noOverflow(page)
    await screenshot(page, message)
  }
  console.log('PASS: revoked, expired and invalid invitations show actionable errors at 320px')

  await page.goto(`${fixture.origin}/join#${fixture.token}`)
  await visible(page, `加入「${fixture.name}」`)
  await page.getByRole('button', { name: '暂不加入，返回普通登录', exact: true }).click()
  await page.reload()
  await visible(page, '试用邀请码')
  assert.equal(await page.evaluate(() => sessionStorage.getItem('omr:pending-invitation')), null)
  console.log('PASS: cancellation restores normal login without carrying the invitation')
  assert.deepEqual(errors, [], 'browser runtime errors')
  await context.close()
} finally { await browser.close() }
