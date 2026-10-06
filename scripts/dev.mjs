import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const npmCli = process.env.npm_execpath
if (!npmCli) throw new Error('Run this through npm run dev or npm run dev:live')
const demo = process.argv.includes('--demo')
console.log('ForgeFlow: starting Go backend; waiting for http://127.0.0.1:8080/api/health…')
const children = []
let stopping = false
const startup = new AbortController()
function stop(code = 0) {
  if (stopping) return
  stopping = true
  startup.abort()
  for (const child of children) {
    if (process.platform === 'win32' && child.pid) spawn('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore', windowsHide: true })
    else child.kill('SIGTERM')
  }
  process.exitCode = code
}
function launch(command, args, cwd) {
  const child = spawn(command, args, { cwd, stdio: 'inherit' })
  children.push(child)
  child.on('error', error => { console.error(error.message); stop(1) })
  child.on('exit', code => { if (!stopping) stop(code ?? 1) })
  return child
}
process.on('SIGINT', () => stop())
process.on('SIGTERM', () => stop())

try {
  let occupied = false
  try {
    const existing = await fetch('http://127.0.0.1:8080/api/health', {
      signal: AbortSignal.any([startup.signal, AbortSignal.timeout(750)]),
    })
    occupied = true
    await existing.body?.cancel()
  } catch { /* No HTTP backend is listening yet. */ }
  if (occupied) throw new Error('Port 8080 is already serving a backend. Stop the existing dev command before starting another.')
  if (!stopping) launch('go', ['run', './apps/server', ...(demo ? ['--demo'] : [])], root)
  const deadline = Date.now() + 120_000
  let ready = false
  while (!stopping && Date.now() < deadline) {
    try {
      const response = await fetch('http://127.0.0.1:8080/api/health', {
        signal: AbortSignal.any([startup.signal, AbortSignal.timeout(1500)]),
      })
      if (response.ok) {
        const health = await response.json()
        ready = health.status === 'ok' && health.mode === (demo ? 'demo' : 'live')
      }
    } catch { /* A compiling backend has not bound its port yet. */ }
    if (ready || stopping) break
    await new Promise(resolve => setTimeout(resolve, 500))
  }
  if (!stopping) {
    if (!ready) throw new Error('Go backend did not become ready within two minutes; inspect its output above.')
    console.log('ForgeFlow: backend ready. Starting frontend · ' + (demo ? 'explicit DEMO mode' : 'live GitHub discovery'))
    launch(process.execPath, [npmCli, 'run', 'dev'], path.join(root, 'apps/web'))
  }
} catch (error) {
  if (!stopping) { console.error(error.message); stop(1) }
}

