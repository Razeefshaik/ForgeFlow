// Bounded, disposable host diagnosis. Does not select a mode or run a model.
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
const root = path.resolve(import.meta.dirname, '..')
const evidence = path.join(root, '.cache/windows-execution-validation')
fs.mkdirSync(evidence, { recursive: true })
const base = fs.mkdtempSync('S:/StudyResource/TechBoooo/Backend/Contriss/.forgeflow-diagnosis-')
const repo = path.join(base, 'repo')
fs.mkdirSync(repo)
const binary = 'C:/Users/RAZEEF/.codex/packages/app-server-daemon/releases/0.162.1-x86_64-pc-windows-msvc/bin/codex.exe'
function run(name, program, args) {
  const start = new Date().toISOString()
  const result = spawnSync(program, args, {cwd: repo, encoding: 'utf8', timeout: 30000, windowsHide: true})
  const record = {name, start, end: new Date().toISOString(), program, args, status: result.status, error: result.error?.message, output: result.stdout + result.stderr}
  fs.appendFileSync(path.join(evidence, 'diagnosis.jsonl'), JSON.stringify(record) + '\n')
  console.log(JSON.stringify(record))
  return result
}
try {
  run('init', 'git', ['init', '--quiet'])
  run('host-git-shell', 'git', ['-c', 'alias.ffprobe=!exit 0', 'ffprobe'])
  for (const mode of (process.argv.includes('--legacy') ? ['unelevated'] : ['mxc', 'elevated'])) {
    const args = ['sandbox', '-C', repo, '--permission-profile', 'forgeflow', '-c', 'permissions.forgeflow.filesystem={":root"="read",":workspace_roots"={"."="write"}}', '-c', 'permissions.forgeflow.network.enabled=false', '-c', 'approval_policy="never"', '-c', 'features.prefer_mxc=false', '-c', `windows.sandbox="${mode}"`]
    if (mode === 'elevated') args.push('-c', 'notify=[]', '-c', 'mcp_servers={}', '-c', 'mcp_servers.node_repl.enabled=false', '-c', 'features.plugins=false', '-c', 'features.apps=false')
    run(mode + '-git-shell', binary, [...args, '--', 'git', '-c', 'alias.ffprobe=!exit 0', 'ffprobe'])
  }
} finally {
  const resolved = fs.realpathSync(base)
  if (path.dirname(resolved).toLowerCase() !== path.resolve('S:/StudyResource/TechBoooo/Backend/Contriss').toLowerCase() || !path.basename(resolved).startsWith('.forgeflow-diagnosis-')) throw Error('cleanup boundary mismatch')
  fs.rmSync(resolved, {recursive: true})
  fs.appendFileSync(path.join(evidence, 'diagnosis.jsonl'), JSON.stringify({cleanup: resolved, removed: !fs.existsSync(resolved)}) + '\n')
}
