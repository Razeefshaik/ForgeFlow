# Contribution workflow

Discovery and ranking do not execute a contribution. Fresh preview binds approval to issue/repository/configuration/base commit and workspace root. Approval records the inputs before clone/branch preparation. Preparation-only legacy records await explicit coding approval.

PLANNING saves JSON/Markdown before CODING. Optional human plan approval pauses here. TESTING records actual sandboxed commands, outputs, exit codes and durations. Failures enter bounded FIXING and retest. A fresh read-only REVIEWING context examines the issue, plan, diff and actual evidence. REQUEST_CHANGES enters bounded FIXING and retest/review. Passing verification plus APPROVE yields READY and a report.

The [issue reactor](issue-reactor.md) classifies setup failures before the code-fix loop. Approved dependency retries and locked npm installation have a separate three-action session budget. A verification-blocked contributor with a patch can proceed to control-plane checks; its summary never counts as a test result. The workspace shows the current diagnosis, with saved output and an explicit recovery action for stopped work. Each failed check and recovery stays in the audit history.

Pause/stop cancels the owned process tree and retains source/evidence; restart interrupts running work into BLOCKED. Resume is explicit. Constraints may be changed before execution or while paused/blocked, and are audited. Abandon preserves the workspace and history.

READY can prepare a local commit and PR title/body. This does not push. PR_PREPARED requires a separate approval bound to commit/title/body before a fixed fork branch push and GitHub PR creation. Changed commits or dirty workspaces are refused. Existing matching open PRs are reused on retry. No merge action exists.
