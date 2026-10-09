import assert from 'node:assert/strict'
import { test, type TestContext } from 'node:test'
import { preparePhoto } from '../photo.ts'

// mockImage supplies device decoding and JPEG output without requiring a browser.
function mockImage(t: TestContext, outputBytes = 128) {
  const source = { src: '', naturalWidth: 2400, naturalHeight: 3200, decode: async () => {} }
  const canvas = {
    width: 0,
    height: 0,
    getContext: () => context,
    toBlob: (callback: BlobCallback) => callback(new Blob([new Uint8Array(outputBytes)])),
  }
  const context = {
    fillStyle: '',
    fillRect: () => {},
    drawImage: t.mock.fn(),
  }
  t.mock.method(URL, 'createObjectURL', () => 'blob:photo')
  t.mock.method(URL, 'revokeObjectURL', () => {})
  for (const [name, value] of Object.entries({
    Image: function () {
      return source
    },
    document: { createElement: () => canvas },
  })) {
    const original = Object.getOwnPropertyDescriptor(globalThis, name)
    Object.defineProperty(globalThis, name, { configurable: true, value })
    t.after(() => {
      if (original) Object.defineProperty(globalThis, name, original)
      else Reflect.deleteProperty(globalThis, name)
    })
  }
  return { source, context }
}

test('original and compressed photos up to and including 6 MiB are accepted', async t => {
  const { context } = mockImage(t, 6 * 1024 * 1024)
  for (const size of [2 * 1024 * 1024 + 1, 6 * 1024 * 1024]) {
    const original = new File([new Uint8Array(size)], 'original.png', { type: 'image/png' })
    const prepared = await preparePhoto(original)
    assert.equal(prepared.name, 'photo.jpg')
    assert.equal(prepared.type, 'image/jpeg')
    assert.equal(prepared.size, 6 * 1024 * 1024)
  }
  assert.deepEqual(context.drawImage.mock.calls[0]?.arguments.slice(1), [0, 0, 1200, 1600])
})

test('photos over 6 MiB are rejected before decoding', async t => {
  const { source } = mockImage(t)
  const decode = t.mock.method(source, 'decode')
  const original = new File([new Uint8Array(6 * 1024 * 1024 + 1)], 'large.png', { type: 'image/png' })
  await assert.rejects(preparePhoto(original), /不超过 6 MB/)
  assert.equal(decode.mock.callCount(), 0)
})

test('compressed uploads cannot exceed 6 MiB', async t => {
  mockImage(t, 6 * 1024 * 1024 + 1)
  const original = new File(['image'], 'original.png', { type: 'image/png' })
  await assert.rejects(preparePhoto(original), /压缩后仍超过 6 MB/)
})
