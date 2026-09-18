import assert from 'node:assert/strict'
import { afterEach, mock, test } from 'node:test'
import { ApiError, request, send } from '../request.ts'

afterEach(() => mock.restoreAll())

test('uses same-origin API and unwraps a successful response', async () => {
  mock.method(globalThis, 'fetch', async (url: string, options: RequestInit) => {
    assert.equal(url, '/api/v1/status')
    assert.equal(options.credentials, 'same-origin')
    assert.equal(new Headers(options.headers).get('Accept'), 'application/json')
    return Response.json({ code: 0, data: { name: 'one-more-round' } })
  })
  assert.deepEqual(await request('/status'), { name: 'one-more-round' })
})

test('does not treat HTTP 200 with a failed business code as success', async () => {
  mock.method(globalThis, 'fetch', async () => Response.json({ code: 409, message: '记录已修改' }))
  await assert.rejects(request('/rounds/1'), (error: unknown) => {
    assert.ok(error instanceof ApiError)
    assert.equal(error.status, 200)
    assert.equal(error.code, 409)
    assert.equal(error.message, '记录已修改')
    return true
  })
})

test('rejects HTTP errors even when the business code is zero', async () => {
  mock.method(globalThis, 'fetch', async () => Response.json({ code: 0, data: {} }, { status: 503 }))
  await assert.rejects(request('/status'), { status: 503 })
})

test('reports HTML proxy responses and malformed envelopes', async () => {
  for (const response of [new Response('<html>error</html>'), Response.json({}), Response.json({ code: 0 })]) {
    mock.method(globalThis, 'fetch', async () => response)
    await assert.rejects(request('/status'), ApiError)
    mock.restoreAll()
  }
})

test('reports network failures without exposing implementation errors', async () => {
  mock.method(globalThis, 'fetch', async () => { throw new TypeError('fetch failed') })
  await assert.rejects(request('/status'), { status: 0, message: '连接中断或超时，请重试' })
})


test('JSON writes preserve concurrency headers, decimal strings, zero and null', async () => {
  const round = { version: 3, scores: { a: '-1.2500', b: '0', c: null } }
  mock.method(globalThis, 'fetch', async (url: string, options: RequestInit) => {
    assert.equal(url, '/api/v1/groups/g/rounds/r')
    assert.equal(options.method, 'PUT')
    assert.equal(new Headers(options.headers).get('Idempotency-Key'), 'same-attempt')
    assert.equal(new Headers(options.headers).get('Content-Type'), 'application/json')
    assert.deepEqual(JSON.parse(String(options.body)), round)
    return Response.json({ code: 0, data: round })
  })
  assert.deepEqual(await send('/groups/g/rounds/r', round, 'PUT', 'same-attempt'), round)
})

test('multipart uploads keep browser-generated boundaries and explicit timeout', async () => {
  const body = new FormData()
  body.append('photo', new Blob(['photo']), 'test.png')
  const signal = AbortSignal.timeout(60_000)
  mock.method(globalThis, 'fetch', async (_url: string, options: RequestInit) => {
    assert.equal(new Headers(options.headers).has('Content-Type'), false)
    assert.equal(options.body, body)
    assert.equal(options.signal, signal)
    return Response.json({ code: 0, data: { id: 'photo' } })
  })
  await request('/groups/g/photos', { method: 'POST', body, signal })
})
