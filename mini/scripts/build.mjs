import { readFile, readdir, mkdir, rm, writeFile, copyFile } from 'node:fs/promises'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import ts from 'typescript'
import * as sass from 'sass'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const source = join(root, 'miniprogram')
const target = join(root, 'dist/miniprogram')
const templates = []
const config = JSON.parse(await readFile(join(source, 'app.json'), 'utf8'))
await rm(join(root, 'dist'), { recursive: true, force: true })
await mkdir(target, { recursive: true })
// walk compiles the native source into a developer-tool-independent bundle.
async function walk(directory) {
  for (const item of await readdir(directory, { withFileTypes: true })) {
    const file = join(directory, item.name)
    if (item.isDirectory()) {
      await walk(file)
      continue
    }
    const path = relative(source, file)
    if (file.endsWith('.wxml')) {
      const wxml = await readFile(file, 'utf8')
      if (/wx:(?:if|elif)="(?!\{\{)/.test(wxml)) throw new Error(`Unbound WXML condition: ${path}`)
      templates.push(path)
    }
    const output = join(target, path.replace(/\.ts$/, '.js').replace(/\.scss$/, '.wxss'))
    await mkdir(dirname(output), { recursive: true })
    if (file.endsWith('.ts')) {
      const result = ts.transpileModule(await readFile(file, 'utf8'), {
        compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS },
      })
      await writeFile(output, result.outputText)
    } else if (file.endsWith('.scss')) {
      await writeFile(output, sass.compile(file, { style: 'compressed' }).css)
    } else await copyFile(file, output)
  }
}
await walk(source)
for (const page of config.pages)
  for (const extension of ['.js', '.json', '.wxml', '.wxss']) await readFile(join(target, page + extension))
const project = JSON.parse(await readFile(join(root, 'project.config.json'), 'utf8'))
project.setting.useCompilerPlugins = []
project.miniprogramRoot = 'miniprogram/'
project.srcMiniprogramRoot = 'miniprogram/'
await writeFile(join(root, 'dist/project.config.json'), JSON.stringify(project, null, 2) + '\n')
// Validate WXML using WeChat's own compiler when it is installed locally.
const wcc =
  process.env.WECHAT_WCC ||
  '/Applications/wechatwebdevtools.app/Contents/Resources/app.asar.unpacked/node_modules/wcc-exec/wcc'
const result = spawnSync(wcc, ['-o', join(root, 'dist/templates.js'), ...templates], { cwd: target, encoding: 'utf8' })
if (result.error?.code === 'ENOENT')
  console.log('WXML compiler unavailable; native template verification requires WeChat Developer Tools.')
else if (result.status !== 0) {
  console.error(result.stderr || result.stdout)
  process.exit(1)
} else console.log('WeChat WXML compiler: passed')
console.log(`Built ${config.pages.length} native pages into mini/dist`)
