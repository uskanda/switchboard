日本語: [SECURITY.ja.md](SECURITY.ja.md)

# Security design

## 1. What we protect

- The source code, credentials, and execution environment on each host
- The forge tokens
- The legitimacy of the instructions given to Claude Code (it acts on no one's instructions but yours)

## 2. Trust boundaries

| Zone | Trust | Reason |
|---|---|---|
| Each host (agent, Claude Code) | Trusted | Under your control |
| API responses from the forge (GitHub/GitLab) | Conditional | Authentication and authorization are left to the forge, but third parties can write the text |
| Issue text and comments | Untrusted (data) | Only what passes the author check is taken as instructions |
| Relay server | Close to untrusted | Designed so that, even when compromised, it can pass on nothing but IDs |
| Webhook sender | Decided by the signature check | Anything that fails the check is dropped |

## 3. Threats and countermeasures

| # | Threat | Countermeasure |
|---|---|---|
| T1 | A third party comments on an issue and steers the agent through prompt injection | Comments by anyone outside `trusted_authors` are never treated as instructions. The prompt also marks them off as "reference data". The body, title, and comments are not taken unless the person who last edited them is also trusted. Public repositories are supported too (ADR-062); for them, `include_untrusted` is refused and untrusted comments stay out of the prompt. CI logs and the output of a failed push reach a fix run as untrusted data inside nonce-tagged delimiters, and the allowed tools are not extended (ADR-041). A run that fixes a conflict with the base branch receives only the pull or merge request number and the base branch name (ADR-044) |
| T2 | Injection of arbitrary instructions through a compromised relay server | The relay passes on only IDs, and the local agent fetches the text again from the forge. No forge token is stored on the relay |
| T3 | Forged webhooks | Signature check (constant-time comparison). A failed check is logged and the webhook is dropped |
| T4 | Redelivered or replayed webhooks | Deduplication by delivery ID. The agent also records the IDs of the comments it has processed |
| T5 | Direct attacks on a host (through an inbound port) | Hosts open no listening ports, and the connection to the relay is always outbound. The exception is the web inbox, which listens only on loopback, private, and link-local addresses (an IPv4-mapped address is converted back before the check). Every request, reads included, has its `Host` checked, so DNS rebinding cannot be used to read the list. A reply POST also has its `Origin` checked. A POST that creates an issue gets the same checks (ADR-040). So does a POST that merges the pull or merge request of a done issue, and it merges only the commit shown on screen (ADR-043). Users are authenticated only when `inbox.auth` is set, by logging in with GitLab OpenID Connect (ADR-036, ADR-059). The Android app's registration and approval POSTs get the same checks. The app sends the web inbox URL it is configured with as its `Origin`. An approval is accepted only when the wait's start time carried in the notification matches the latest waiting post (ADR-047). When the relay's webhook endpoint receives from a GitLab on another machine in the LAN, it too listens on a LAN address, never on all interfaces. In that setup, GitLab webhooks are verified only by the signing token (HMAC and timestamp), and `gitlab_token`, which sends the secret in plain text, is refused. The source address is not restricted, and payloads that cross the LAN in plain text are accepted. The relay passes on only IDs, and the agent fetches everything again from the forge (ADR-004, ADR-060) |
| T6 | Leaked forge tokens | Least privilege (read/write on the issues of the target repositories, and only the contents access that is needed). Use a fine-grained PAT or a Project Access Token, stored in the OS keychain |
| T7 | Destructive actions by a runaway agent | A fixed set of tools passed to the run (ADR-052), explicitly allowed tools, work in a worktree, and timeouts. Only the agent pushes, and only to the issue's branch; Claude's environment gets no credentials. What stops pushes to main, merges, and releases is the sandbox and the checks before a push on protected paths, secrets, and the branch (ADR-063). The protected paths also cover `CLAUDE.md`, `AGENTS.md`, and `.claude/` in subdirectories, and the original path of a rename. Tools the user adds (`allow_tools_extra`) can be written only in the config file; the inbox API and the web inbox cannot change them. When the config is loaded, an unrestricted `Bash` entry and any entry that names a denied operation (`git push`, `git rebase`, `gh`, `WebFetch`) are errors. A deny rule is not a boundary (V5), so withholding the forge credentials (ADR-012) is the barrier (ADR-045). Shell commands run in a per-run sandbox: writes are limited to the worktree and `$TMPDIR`, and the network is unreachable. The places that hold credentials, and switchboard's config and state directories, cannot be read. Only `git` and the verify commands run outside the sandbox, and on a host that cannot use the sandbox, claude is not started (ADR-057) |
| T8 | Spoofed hidden markers that confuse the inbox | Markers are taken only from comments posted by the agent's account (or by you) |
| T11 | Impersonation among the people who share the inbox, and viewing projects one has no right to see | With `inbox.auth` set, writes to GitLab use the OAuth token of the person logged in, never a shared token. On GitHub, only the owner can view and write. The list and the single-issue view show only the projects that person can read with their own token. The agent acts as a bot identity and is not in `trusted_authors` (ADR-061) |
| T10 | Sniffing of the web inbox session cookie on the same LAN | For now the web inbox serves plain HTTP, so the cookie travels in plain text. Whoever steals it can act as that user. The cookie holds only a session ID, and GitLab tokens never leave the inbox service. A session ends after 24 hours without use or after 7 days. HTTPS is tracked in #155 (ADR-059) |
| T9 | Leaking secrets to the outside through issue text | An instruction to keep secrets out of the output (comments), and where possible a pattern check before posting. An FCM push notification carries only the issue reference, the waiting reason, the title, times, and the pull or merge request number and commit; it never carries the text of questions, plans, or comments (ADR-047) |
| T12 | A diagram in a plan question that restyles the web inbox, to hide or fake its options | switchboard writes a diagram only of three Mermaid kinds and drops one with `classDef`, `style`, `linkStyle`, `click`, or a `%%{` directive (the drop is in the audit log). The web inbox checks the same again, draws only the diagram of the latest question with Mermaid's strict mode, and draws any other Mermaid text as code. Its CSP allows inline styles for these diagrams, but scripts and every load stay on its own origin (ADR-066) |

## 4. Tokens and permissions

- GitHub: a fine-grained PAT limited to the target repositories, with Issues: Read/Write, Pull requests: Read/Write, and Contents: Read/Write (as much as opening pull requests needs). To watch the CI of a pull request, it also has Actions: Read and Commit statuses: Read (ADR-041). A conflict between a pull request and its base branch is read with `Pull requests: Read`, so no permission is added (ADR-044).
- GitLab: the `api` scope of a Project Access Token or a PAT (when the scope cannot be narrowed, a Project Access Token limited to the target project is recommended).
- The inbox token (`inbox_token_ref`) carries a person's identity and is kept separate from the agent's token (ADR-008). On GitHub it gets read/write on Issues, Pull requests, and Contents. The write access to Pull requests and Contents lets the web inbox merge the pull request of a done issue (ADR-043). It also gets Actions: Read and Commit statuses: Read, read-only permissions that put the CI results in the merge confirmation dialog. Without them, the dialog only leaves out the CI (ADR-050). On GitLab the `api` scope is enough.
- The inbox token also reads the project icons shown in the web inbox. This needs no new permission, because the token reads only the owner's public profile on GitHub and only project and group avatars on GitLab. The token is sent only to the forge API host and the GitLab instance host. It is never sent to the GitHub avatar image host, to an `icon` URL in the config, or to another host a redirect leads to. Images are limited to PNG, JPEG, GIF, and WebP of up to 256 KiB, and SVG is not accepted. Because switchboard-web serves the images from its own origin, its CSP still loads images only from that origin (`default-src 'self'`, ADR-055).
- When `inbox.notify.fcm` is set, the inbox service reads the Firebase service account key from Secret Service through `credentials_ref`. The key has only the permission to send through FCM, and it is not given to the agent service (ADR-047).
- The relay server holds only the webhook secret. An empty secret is an error at startup and at verification.
- `switchboard doctor` checks the permissions of the tokens. It reads the scopes of a GitLab token: no `api` fails, and `sudo` and the like warn. For a GitHub classic PAT it reads `X-OAuth-Scopes`: neither `repo` nor `public_repo` fails, and the whole of `repo`, `admin:*`, and the like warn. A fine-grained PAT's permissions cannot be read, so doctor only tries to read issues, pull requests, and contents. It does not try writing, which would have side effects.

## 5. Audit

- The agent writes a local log of the received events, the check results (taken or dropped, and why), the commands it ran, and their exit codes.
- The comment history of each issue is the history of operations.

## 6. Unverified behavior (linked to the verification items in ROADMAP.md)

- What happens when an interactive session and `--resume` overlap on the same session ID
- Whether the Notification hook can tell "waiting for permission" from "waiting for input" (it does not fire under `-p`, so hooks are not used; V2, V13, ADR-063)
- What happens when a headless run needs a permission prompt (whether it stops or fails)
