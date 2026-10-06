import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import fs from 'node:fs'
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit' })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}
run('go', ['test', './...'])
run('go', ['vet', './...'])
fs.mkdirSync(path.join(root, 'bin'), { recursive: true })
run('go', ['build', '-o', path.join('bin', process.platform === 'win32' ? 'forgeflow.exe' : 'forgeflow'), './apps/server'])
if (!process.env.npm_execpath) throw new Error('Run npm run build')
run(process.execPath, [process.env.npm_execpath, 'run', 'build'], path.join(root, 'apps/web'))

