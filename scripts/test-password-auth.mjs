// Invoked by TestPasswordAuthBrowser against its disposable real API/database.
import assert from 'node:assert/strict'
import { join } from 'node:path'

const { chromium } = await import(process.env.OMR_PLAYWRIGHT_MODULE || 'playwright')
const fixture = JSON.parse(process.env.OMR_BROWSER_FIXTURE)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []

// field uses the leading visible label because field hints join accessible names.
function field(page, name) { return page.getByLabel(new RegExp(`^${name}`)).first() }
// checkLayouts verifies all supported widths and captures optional review artifacts.
async function checkLayouts(page, name) {
  for (const width of [320, 390, 1280]) {
    await page.setViewportSize({ width, height: width === 1280 ? 900 : 844 })
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, `${name} overflows at ${width}px`)
    if (name !== 'password-set') {
      assert.equal(await page.locator('.j-auth').count(), 1, `${name} renders duplicate authentication panels`)
      const panel = await page.locator('.j-auth').boundingBox()
      const height = await page.evaluate(() => document.documentElement.scrollHeight)
      console.log(`LAYOUT: ${name} width=${width} document-height=${height} panel-height=${Math.round(panel.height)}`)
    }
    if (process.env.OMR_BROWSER_ARTIFACTS) await page.screenshot({ path: join(process.env.OMR_BROWSER_ARTIFACTS, `${name}-${width}.png`), fullPage: true, animations: 'disabled' })
  }
  await page.setViewportSize({ width: 390, height: 844 })
}
// context creates a clean account/device boundary for each independent flow.
async function context() {
  const value = await browser.newContext({ viewport: { width: 390, height: 844 } })
  const page = await value.newPage()
  page.setDefaultTimeout(10000)
  page.on('pageerror', error => errors.push(error.message))
  return [value, page]
}
// fillRegistration preserves the password exactly, including its trailing space.
async function fillRegistration(page, username, invite) {
  await field(page, '用户名').fill(username)
  await field(page, '密码').fill(fixture.password)
  await field(page, '确认密码').fill(fixture.password)
  if (invite) await field(page, '试用邀请码').fill(invite)
}
// logout exercises the account menu rather than clearing browser cookies manually.
async function logout(page) {
  await page.getByRole('button', { name: '我的账号', exact: true }).click()
  await page.getByRole('button', { name: '退出登录', exact: true }).click()
  await page.waitForURL('**/login')
}
// me reads the real persisted account associated with the browser's HttpOnly cookie.
async function me(value) {
  const response = await value.request.get(`${fixture.origin}/api/v1/me`)
  assert.equal(response.status(), 200)
  return (await response.json()).data
}
// noStoredPassword checks both durable browser stores for accidental secret recovery.
async function noStoredPassword(page) {
  assert.equal(await page.evaluate(secret => {
    for (const storage of [localStorage, sessionStorage]) {
      for (let i = 0; i < storage.length; i++) {
        if ((storage.getItem(storage.key(i)) || '').includes(secret)) return false
      }
    }
    return true
  }, fixture.password), true, 'password persisted in browser storage')
}

try {
  const [trialContext, trialPage] = await context()
  await trialPage.goto(`${fixture.origin}/login`)
  await field(trialPage, '邮箱').waitFor()
  await trialPage.getByRole('link', { name: '用账号密码注册', exact: true }).click()
  await trialPage.waitForURL('**/register')
  await fillRegistration(trialPage, 'browser-player', fixture.trial)
  await checkLayouts(trialPage, 'password-register')
  await noStoredPassword(trialPage)
  await trialPage.reload()
  assert.equal(await field(trialPage, '密码').inputValue(), '')
  assert.equal(await field(trialPage, '确认密码').inputValue(), '')
  await fillRegistration(trialPage, 'browser-player', fixture.trial)
  assert.equal(Array.from(fixture.password).length, 8, 'password fixture must cover the minimum length')
  await field(trialPage, '密码').fill('1234567')
  await field(trialPage, '确认密码').fill('1234567')
  await trialPage.getByRole('button', { name: '注册并登录', exact: true }).click()
  await trialPage.getByRole('alert').filter({ hasText: '8–128' }).waitFor()
  assert.equal(new URL(trialPage.url()).pathname, '/register')
  await fillRegistration(trialPage, 'browser-player', fixture.trial)
  await field(trialPage, '确认密码').fill('different-password-value')
  await trialPage.getByRole('button', { name: '注册并登录', exact: true }).click()
  await trialPage.getByRole('alert').waitFor()
  assert.equal(new URL(trialPage.url()).pathname, '/register')
  await field(trialPage, '确认密码').fill(fixture.password)
  await field(trialPage, '试用邀请码').fill('invalid-browser-trial')
  const invalidAdmission = trialPage.waitForResponse(response => response.url().endsWith('/api/v1/auth/password/register'))
  await trialPage.getByRole('button', { name: '注册并登录', exact: true }).click()
  assert.equal((await invalidAdmission).status(), 403)
  await trialPage.getByRole('alert').waitFor()
  assert.equal(await field(trialPage, '密码').inputValue(), fixture.password)
  assert.equal(await field(trialPage, '用户名').inputValue(), 'browser-player')
  await field(trialPage, '试用邀请码').fill(fixture.trial)
  await trialPage.getByRole('button', { name: '注册并登录', exact: true }).click()
  await trialPage.waitForURL('**/join')
  assert.equal((await me(trialContext)).username, 'browser-player')
  await logout(trialPage)
  console.log('PASS: eight-character trial registration, seven-character rejection, confirmation validation, refresh secret clearing and logout')

  const [duplicateContext, duplicatePage] = await context()
  await duplicatePage.goto(`${fixture.origin}/register`)
  await fillRegistration(duplicatePage, 'browser-player', fixture.duplicateTrial)
  const duplicateResponse = duplicatePage.waitForResponse(response => response.url().endsWith('/api/v1/auth/password/register'))
  await duplicatePage.getByRole('button', { name: '注册并登录', exact: true }).click()
  assert.equal((await duplicateResponse).status(), 409)
  await duplicatePage.getByRole('alert').waitFor()
  assert.equal(await field(duplicatePage, '用户名').inputValue(), 'browser-player')
  assert.equal(await field(duplicatePage, '密码').inputValue(), fixture.password)
  assert.equal(await field(duplicatePage, '试用邀请码').inputValue(), fixture.duplicateTrial)
  await field(duplicatePage, '用户名').fill('browser-corrected-player')
  await duplicatePage.getByRole('button', { name: '注册并登录', exact: true }).click()
  await duplicatePage.waitForURL('**/join')
  assert.equal((await me(duplicateContext)).username, 'browser-corrected-player')
  await duplicateContext.close()
  console.log('PASS: duplicate username preserves input and admission for correction')

  await trialPage.getByRole('button', { name: '账号密码', exact: true }).click()
  await field(trialPage, '用户名或邮箱').fill('browser-player')
  await field(trialPage, '密码').fill(fixture.password)
  await trialPage.getByRole('button', { name: '邮箱验证码', exact: true }).click()
  await trialPage.getByRole('button', { name: '账号密码', exact: true }).click()
  assert.equal(await field(trialPage, '密码').inputValue(), '')
  await field(trialPage, '密码').fill(fixture.password)
  await noStoredPassword(trialPage)
  await trialPage.reload()
  await field(trialPage, '邮箱').waitFor()
  await trialPage.getByRole('button', { name: '账号密码', exact: true }).click()
  assert.equal(await field(trialPage, '密码').inputValue(), '')
  await checkLayouts(trialPage, 'password-login')
  await field(trialPage, '用户名或邮箱').fill('browser-player')
  await field(trialPage, '密码').fill('a-wrong-password-value')
  const wrongResponse = trialPage.waitForResponse(response => response.url().endsWith('/api/v1/auth/password/login'))
  await trialPage.getByRole('button', { name: '登录', exact: true }).click()
  assert.equal((await wrongResponse).status(), 401)
  await trialPage.getByRole('alert').waitFor()
  assert.equal(await field(trialPage, '用户名或邮箱').inputValue(), 'browser-player')
  assert.equal(await field(trialPage, '密码').inputValue(), 'a-wrong-password-value')
  await field(trialPage, '密码').fill(fixture.password)
  await trialPage.getByRole('button', { name: '登录', exact: true }).click()
  await trialPage.waitForURL('**/join')
  assert.equal((await me(trialContext)).username, 'browser-player')
  await trialContext.close()
  console.log('PASS: login mode switching, secret clearing, wrong-password retry and existing login')

  const [groupContext, groupPage] = await context()
  await groupPage.goto(`${fixture.origin}/join#${fixture.token}`)
  await groupPage.getByText(`加入「${fixture.groupName}」`, { exact: false }).waitFor()
  await groupPage.getByRole('link', { name: '用账号密码注册', exact: true }).click()
  await groupPage.waitForURL('**/register')
  await groupPage.reload()
  await groupPage.getByText(`加入「${fixture.groupName}」`, { exact: false }).waitFor()
  assert.equal(await groupPage.getByLabel(/^试用邀请码/).count(), 0)
  await fillRegistration(groupPage, 'browser-group-player')
  await checkLayouts(groupPage, 'password-group-register')
  await groupPage.getByRole('button', { name: '注册并登录', exact: true }).click()
  await groupPage.waitForURL('**/group')
  const groupUser = await me(groupContext)
  const snapshot = await (await groupContext.request.get(`${fixture.origin}/api/v1/groups/${fixture.groupID}`)).json()
  assert.equal(snapshot.data.members.find(member => member.user_id === groupUser.id).username, 'browser-group-player')
  assert.equal(await groupPage.evaluate(() => sessionStorage.getItem('omr:pending-invitation')), null)
  await groupPage.getByText('browser-group-player', { exact: true }).first().waitFor()
  await groupContext.close()
  console.log('PASS: group invitation survives register-route refresh and atomically joins with username display')

  const [emailContext, emailPage] = await context()
  const email = 'browser-existing-email@example.test'
  await emailPage.goto(`${fixture.origin}/login`)
  await field(emailPage, '邮箱').fill(email)
  await field(emailPage, '试用邀请码').fill(fixture.emailTrial)
  await emailPage.getByRole('button', { name: '发送验证码', exact: true }).click()
  await field(emailPage, '邮箱验证码').waitFor()
  await emailPage.reload()
  assert.equal(await field(emailPage, '邮箱').inputValue(), email)
  const inbox = await (await emailContext.request.get(`${fixture.origin}/__test__/code?email=${encodeURIComponent(email)}`)).json()
  await field(emailPage, '邮箱验证码').fill(inbox.code)
  await emailPage.getByRole('button', { name: '登录', exact: true }).click()
  await emailPage.waitForURL('**/join')
  const originalUser = await me(emailContext)
  await field(emailPage, '小组名称').fill('邮箱原来的小组')
  await field(emailPage, '你的玩家昵称').fill('邮箱玩家')
  await emailPage.getByRole('button', { name: '创建小组', exact: true }).click()
  await emailPage.waitForURL(fixture.origin + '/')
  await emailPage.getByRole('button', { name: '我的账号', exact: true }).click()
  await emailPage.getByRole('button', { name: '设置密码', exact: true }).click()
  await emailPage.getByRole('heading', { name: '设置密码', exact: true }).waitFor()
  await field(emailPage, '用户名').fill('browser-email-player')
  await field(emailPage, '新密码').fill('1234567')
  await field(emailPage, '确认密码').fill('1234567')
  await emailPage.getByRole('button', { name: '保存密码', exact: true }).click()
  await emailPage.getByRole('alert').filter({ hasText: '8–128' }).waitFor()
  assert.equal((await me(emailContext)).id, originalUser.id)
  await field(emailPage, '用户名').fill('browser-player')
  await field(emailPage, '新密码').fill(fixture.password)
  await field(emailPage, '确认密码').fill(fixture.password)
  const conflictingSetup = emailPage.waitForResponse(response => response.url().endsWith('/api/v1/auth/password/set'))
  await emailPage.getByRole('button', { name: '保存密码', exact: true }).click()
  assert.equal((await conflictingSetup).status(), 409)
  await emailPage.getByRole('alert').waitFor()
  assert.equal(await field(emailPage, '新密码').inputValue(), fixture.password)
  assert.equal((await me(emailContext)).id, originalUser.id)
  await field(emailPage, '用户名').fill('browser-email-player')
  await checkLayouts(emailPage, 'password-set')
  await emailPage.getByRole('button', { name: '保存密码', exact: true }).click()
  await emailPage.getByText('密码已设置。', { exact: false }).waitFor()
  const passwordUser = await me(emailContext)
  assert.equal(passwordUser.id, originalUser.id)
  assert.equal(passwordUser.email, email)
  assert.equal(passwordUser.username, 'browser-email-player')
  await emailPage.getByRole('button', { name: '完成', exact: true }).click()
  await logout(emailPage)
  await emailPage.getByRole('button', { name: '账号密码', exact: true }).click()
  await field(emailPage, '用户名或邮箱').fill(email)
  await field(emailPage, '密码').fill(fixture.password)
  await emailPage.getByRole('button', { name: '登录', exact: true }).click()
  await emailPage.waitForURL(fixture.origin + '/')
  assert.equal((await me(emailContext)).id, originalUser.id)
  await emailPage.getByRole('combobox', { name: '当前小组', exact: true }).waitFor()
  await emailPage.getByText('第一次相聚，从这一局开始', { exact: false }).waitFor()
  await noStoredPassword(emailPage)
  await emailContext.close()
  console.log('PASS: original email flow, seven-character rejection, eight-character password setup keeps identity/groups and email password login')

  assert.deepEqual(errors, [], 'browser runtime errors')
} finally {
  await browser.close()
}
