# GitHub browser sign-in

Open **GitHub account** in ForgeFlow, or `/login`. Register a GitHub OAuth app once at https://github.com/settings/applications/new:

- Name: ForgeFlow
- Homepage: `http://127.0.0.1:5173`
- Authorization callback: `http://127.0.0.1:5173/login` (not used by device flow)
- Enable **Device Flow** in the OAuth app settings.

Save the app's **Client ID** on the account page. It is a public identifier; no personal token or client secret is required. An administrator can also supply `FORGEFLOW_GITHUB_CLIENT_ID`. Click **Sign in with GitHub**, open the GitHub device page, enter the displayed one-time code, and approve access. ForgeFlow verifies your account before saving credentials.

The requested scopes are `public_repo offline_access`: public repository contribution permissions and refreshable sessions. Private repository discovery remains unsupported. PR submission requires separate human approval; login does not submit anything or start contribution execution.

On Windows, access and refresh tokens are encrypted using current-user DPAPI and saved under `%APPDATA%/ForgeFlow/<project-hash>/credential.dpapi`. The public Client ID lives beside them. Tokens do not enter browser storage, SQLite, events, source control, or contribution repositories. Saved browser credentials take precedence over environment/CLI credentials. Existing environment and `gh auth login` fallback still work when no saved account exists. Non-Windows browser credential persistence is currently unsupported; environment/CLI authentication remains available.

Sessions restore on startup and expiring tokens refresh automatically before expiry. Successful login updates the running GitHub client and clears the previous credential's rate-limit backoff. Automatic discovery remains paused if you paused it. Signing out clears ForgeFlow's in-memory and saved credential and cancels any pending challenge. It does not revoke the OAuth app at GitHub or delete environment/CLI credentials; use GitHub's application settings to revoke authorization. Environment/CLI credentials may be detected again on the next startup.

Authentication changes create audit events containing the credential source only. Credentials are never included. An already approved PR submission uses a fixed credential snapshot throughout its fork/push/PR operation.

Protocol reference: [GitHub device authorization and refresh](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps).
