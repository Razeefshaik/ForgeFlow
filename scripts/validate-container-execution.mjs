// Explicit live validation: consumes authenticated Codex sessions only in
// disposable clones. Never touches live contribution records or publishes PRs.
import fs from 'node:fs'
import path from 'node:path'
import { spawn } from 'node:child_process'
const root = path.resolve(import.meta.dirname, '..')
const settings = JSON.parse(fs.readFileSync(path.join(root, 'configs/local.json'), 'utf8'))
const base = path.resolve(process.env.FORGEFLOW_CONTRIBUTIONS_DIR || settings.contributions_dir)
const evidence = path.join(root, '.cache/windows-execution-validation', 'proof-' + new Date().toISOString().replaceAll(':', '-'))
fs.mkdirSync(evidence, { recursive: true })
const env = { ...process.env, FORGEFLOW_TEST_CONTAINER: '1', FORGEFLOW_REAL_CODEX_TEST: '1', FORGEFLOW_TEST_SANDBOX: 'mxc', FORGEFLOW_TEST_CONTAINER_IMAGE: process.env.FORGEFLOW_TEST_CONTAINER_IMAGE || 'forgeflow-verification:go1.26-node22', FORGEFLOW_TEST_CONTRIBUTIONS_ROOT: base, FORGEFLOW_TEST_EVIDENCE: evidence }
console.log('Evidence: ' + evidence)
async function run(name, program, args, cwd) {
  const log = fs.createWriteStream(path.join(evidence, name + '.log'), {flags: 'wx'})
  const child = spawn(program, args, { cwd, env, windowsHide: true })
  for (const stream of [child.stdout, child.stderr]) stream.on('data', data => { log.write(data); process.stdout.write(data) })
  const code = await new Promise((resolve, reject) => { child.on('error', reject); child.on('exit', resolve) })
  await new Promise(resolve => log.end(resolve))
  if (code !== 0) throw Error(name + ' exited ' + code + '; evidence retained at ' + evidence)
}
if (!process.argv.includes('--workflow-only')) {
  await run('boundary-edit-test-cancel', path.join(root, 'bin/container-validation.test.exe'), ['-test.run=^TestRealContainerVerification$', '-test.v', '-test.timeout=15m'], path.join(root, 'internal/codex'))
}
await run('persisted-workflow', path.join(root, 'bin/container-workflow.test.exe'), ['-test.run=^TestRealCodexWorkflow$', '-test.v', '-test.timeout=20m'], path.join(root, 'internal/execution'))
console.log('Live validation passed; evidence retained at ' + evidence)
