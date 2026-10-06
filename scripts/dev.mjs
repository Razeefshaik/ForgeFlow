import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const npmCli = process.env.npm_execpath
if (!npmCli) throw new Error('Run this through npm run dev or npm run dev:live')
const demo = process.argv.includes('--demo')
console.log('ForgeFlow: http://127.0.0.1:5173 · ' + (demo ? 'explicit DEMO mode' : 'live GitHub discovery'))
const children = [
  spawn('go', ['run', './apps/server', ...(demo ? ['--demo'] : [])], { cwd: root, stdio: 'inherit' }),
  spawn(process.execPath, [npmCli, 'run', 'dev'], { cwd: path.join(root, 'apps/web'), stdio: 'inherit' }),
]
let stopping = false
function stop(code = 0) {
  if (stopping) return
  stopping = true
  for (const child of children) {
    if (process.platform === 'win32' && child.pid) spawn('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore', windowsHide: true })
    else child.kill('SIGTERM')
  }
  process.exitCode = code
}
for (const child of children) {
  child.on('error', error => { console.error(error.message); stop(1) })
  child.on('exit', code => { if (!stopping) stop(code ?? 1) })
}
process.on('SIGINT', () => stop())
process.on('SIGTERM', () => stop())

