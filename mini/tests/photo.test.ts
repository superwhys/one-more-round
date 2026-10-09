import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createRequire } from 'node:module'
import fs from 'node:fs'
import path from 'node:path'
import ts from 'typescript'
import vm from 'node:vm'

const root = path.resolve(import.meta.dirname, '../miniprogram')
type CallbackOptions = Record<string, any>

// load executes the photo utility and its real dependencies against a WeChat fake.
function load(file: string, wx: object, cache = new Map<string, { exports: unknown }>()) {
  const filename = path.join(root, file.endsWith('.ts') ? file : file + '.ts')
  if (cache.has(filename)) return cache.get(filename)!.exports as Record<string, Function>
  const module = { exports: {} }
  cache.set(filename, module)
  const code = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText
  const require = (name: string) =>
    name.startsWith('.')
      ? load(path.relative(root, path.resolve(path.dirname(filename), name)), wx, cache)
      : createRequire(import.meta.url)(name)
  vm.runInNewContext('(function(require,module,exports,wx){' + code + '\n})', { Error, Promise, Date, Set })(
    require,
    module,
    module.exports,
    wx,
  )
  return module.exports as Record<string, Function>
}

// runtime records image processing and upload while supplying an original file size.
function runtime(size: number) {
  const uploads: CallbackOptions[] = []
  const encodings: CallbackOptions[] = []
  const removed: string[] = []
  const imageReads: string[] = []
  const wx = {
    getFileSystemManager: () => ({
      getFileInfo: (options: CallbackOptions) => options.success({ size }),
      unlink: (options: CallbackOptions) => removed.push(options.filePath),
    }),
    getImageInfo(options: CallbackOptions) {
      imageReads.push(options.src)
      options.success({ type: 'jpeg', width: 4000, height: 3000 })
    },
    createOffscreenCanvas() {
      return {
        createImage: () => ({
          onload: () => {},
          set src(_value: string) {
            this.onload()
          },
        }),
        getContext: () => ({ fillRect() {}, drawImage() {} }),
      }
    },
    canvasToTempFilePath(options: CallbackOptions) {
      encodings.push(options)
      options.success({ tempFilePath: '/tmp/compressed.jpg' })
    },
    uploadFile(options: CallbackOptions) {
      uploads.push(options)
      options.success({ statusCode: 200, data: JSON.stringify({ code: 0, data: { id: 'photo' } }) })
    },
  }
  return { photo: load('utils/photo', wx), uploads, encodings, removed, imageReads }
}

test('an original photo of exactly 6 MiB is compressed and uploaded', async () => {
  const app = runtime(6 * 1024 * 1024)
  assert.equal((await app.photo.uploadPhoto('group', '/tmp/original.jpg')).id, 'photo')
  assert.equal(app.encodings.length, 1)
  assert.equal(app.encodings[0].width, 1600)
  assert.equal(app.encodings[0].height, 1200)
  assert.equal(app.encodings[0].fileType, 'jpg')
  assert.equal(app.encodings[0].quality, 0.85)
  assert.equal(app.uploads.length, 1)
  assert.equal(app.uploads[0].filePath, '/tmp/compressed.jpg')
  assert.ok(app.uploads[0].url.endsWith('/api/v1/groups/group/photos'))
  assert.equal(app.uploads[0].name, 'photo')
  assert.deepEqual(app.removed, ['/tmp/compressed.jpg'])
})

test('an original photo over 6 MiB is rejected before image processing or upload', async () => {
  const app = runtime(6 * 1024 * 1024 + 1)
  await assert.rejects(app.photo.uploadPhoto('group', '/tmp/original.jpg'), /原始照片不能超过 6 MiB/)
  assert.deepEqual(app.imageReads, [])
  assert.deepEqual(app.encodings, [])
  assert.deepEqual(app.uploads, [])
})
