import assert from 'node:assert/strict'
import { afterEach, mock, test } from 'node:test'
import { login, sendLoginCode } from '../auth.ts'

afterEach(() => mock.restoreAll())

test('sending a login code submits only the mailbox', async () => {
  mock.method(globalThis, 'fetch', async (url: string, options: RequestInit) => {
    assert.equal(url, '/api/v1/auth/code')
    assert.deepEqual(JSON.parse(String(options.body)), { email: 'member@example.test' })
    return Response.json({ code: 0, data: null })
  })
  await sendLoginCode('member@example.test')
})

test('trial admission is supplied when consuming the email code', async () => {
  mock.method(globalThis, 'fetch', async (url: string, options: RequestInit) => {
    assert.equal(url, '/api/v1/auth/login')
    assert.deepEqual(JSON.parse(String(options.body)), {
      email: 'new@example.test',
      code: '123456',
      invite: 'trial-token',
      group_token: '',
    })
    return Response.json({ code: 0, data: { id: 'new-member', email: 'new@example.test' } })
  })
  await login('new@example.test', '123456', 'trial-token')
})

test('group admission on login excludes an unrelated trial invitation', async () => {
  mock.method(globalThis, 'fetch', async (_url: string, options: RequestInit) => {
    assert.deepEqual(JSON.parse(String(options.body)), {
      email: 'new@example.test',
      code: '123456',
      invite: '',
      group_token: 'group-token',
    })
    return Response.json({ code: 0, data: { id: 'new-member', group_id: 'group' } })
  })
  assert.equal((await login('new@example.test', '123456', 'stale-trial', 'group-token')).group_id, 'group')
})

test('an existing mailbox can log in without any admission credential', async () => {
  mock.method(globalThis, 'fetch', async (_url: string, options: RequestInit) => {
    assert.deepEqual(JSON.parse(String(options.body)), {
      email: 'member@example.test',
      code: '123456',
      invite: '',
      group_token: '',
    })
    return Response.json({ code: 0, data: { id: 'member', email: 'member@example.test' } })
  })
  assert.equal((await login('member@example.test', '123456')).id, 'member')
})
