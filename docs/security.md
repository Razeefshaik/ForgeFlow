# Security and isolation

The foundation binds numeric loopback interfaces and rejects non-loopback Host values, cross-origin writes, non-JSON writes and oversized request bodies. There is no CORS wildcard. The frontend and API run through one origin or the Vite proxy.
SQLite demo/live mode is fixed at initialization. Secrets are not accepted in configuration. Audit events/config snapshots are immutable; no delete API exists.

The active build is isolated under contributions/forgeflow-foundation/repo with its own Git repository and branch. No enclosing project files were modified.
No external repository commands, agents, cloning, pushing or PR submission are available yet. The future runner must validate resolved paths and symlink/junction boundaries, use timeouts/structured arguments, constrain writable roots through a real sandbox and independently review changes. Setting a working directory alone does not prevent an agent from accessing other files.
The Operator only dispatches supported reads/proposals; it has no shell capability or model connection. GitHub tokens use environment or a bounded CLI lookup and remain in memory. Scans are GET-only, follow no redirects, reject private repositories and never execute repository content. Error bodies are withheld. Cache/credential hashes are memory-only; durable observations contain public evidence rather than credentials.
This service is for a local trusted user and must not be exposed publicly without authentication and additional isolation.

