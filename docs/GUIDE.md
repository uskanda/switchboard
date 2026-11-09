日本語: [GUIDE.ja.md](GUIDE.ja.md)

# switchboard operations guide

This guide covers what comes after installation: registering tokens, configuration, and how to use each feature. The [README](../README.md) gives the overview and the assumptions.

## Registering tokens

Create two GitHub fine-grained personal access tokens and register them in the Secret Service. Never write a token in the config file.

| Use | Config key | Permissions (on the target repositories only) |
|---|---|---|
| Agent (labels, comments, push, PRs, watching CI) | `token_ref` | Read and write on Contents, Issues, and Pull requests. Read on Actions and Commit statuses |
| Inbox (list, replies, merging from the web inbox) | `inbox_token_ref` | Read and write on Issues, Pull requests, and Contents. Read on Actions and Commit statuses |

```bash
secret-tool store --label='switchboard github' service switchboard/github username token
```

```bash
secret-tool store --label='switchboard github inbox' service switchboard/github-inbox username token
```

Both commands read the token from standard input. switchboard never passes the tokens to `claude`.

If the agent's token cannot read Actions and Commit statuses, the agent does not watch the CI of its PRs. Nothing else changes. `switchboard doctor` warns about it.

The inbox token writes to Pull requests and Contents so that you can merge done PRs from the web inbox (ADR-043). Merges happen under your identity, and the forge records each one as your merge. To add these permissions to an existing token, open the token's settings on GitHub, set Pull requests and Contents to "Read and write", and save. The token's value does not change, so you do not need to register it again. Until you add them, only merging fails, with "The inbox token has no permission to merge."

The inbox token reads Actions and Commit statuses to show the CI results in the merge confirmation dialog (ADR-050). Add them the same way, setting Actions and Commit statuses to "Read-only". Until you add them, the dialog says "The inbox token has no permission to read CI", and you can still merge. `switchboard doctor` warns about it.

For GitLab projects, create a GitLab personal access token with the `api` scope and register it the same way. You may register the same token for the agent and for the inbox. This token can act on all of your projects. To limit the agent to one project, use a Project Access Token through the per-project override described below. The fine-grained tokens of GitLab 19.2 and later can also narrow the scope, but switchboard has not checked that they work with every API it uses.

```bash
secret-tool store --label='switchboard gitlab' service switchboard/gitlab username token
```

```bash
secret-tool store --label='switchboard gitlab inbox' service switchboard/gitlab-inbox username token
```

## Configuration

Copy the example, then replace `YOUR_GITHUB_USER`, the host name, and the project paths.

```bash
mkdir -p ~/.config/switchboard
```

```bash
cp config/switchboard.example.yaml ~/.config/switchboard/config.yaml
```

After editing, check the result with `doctor`. It checks the tokens, the user IDs, the repositories of the projects this host is in charge of, and the version of `claude`, in that order. If it finds a problem, it exits with a nonzero code.

```bash
switchboard doctor
```

In Phase 1, the agent also posts as your own account. Put only yourself in `trusted_authors`. Public repositories can be targets too, but you cannot use `include_untrusted` (ADR-062). `claude` runs inside a sandbox under your own OS user; read [SECURITY.md](SECURITY.md) for the details.

`switchboard doctor` also checks the tokens' permissions. For GitLab tokens and GitHub classic PATs, it reads the scopes: a missing scope fails the check, and too broad a scope gives a warning. GitHub fine-grained PATs do not report their permissions, so `doctor` only tries reading each project's issues, PRs, and contents. It cannot check write permissions.

### GitLab

In the example config, the indented comment lines hold the GitLab settings. Uncomment them and fill in `gitlab` under `forges` and `gitlab` under `trusted_authors`. Without `base_url`, switchboard uses `https://gitlab.com`. Write a project as `gl:group/project`; a path with subgroups such as `gl:group/sub/project` works too.

Every user of a GitLab instance can see and comment on its `internal` projects, so switchboard treats them as public projects. `doctor` warns about them and rejects `include_untrusted: true`.

### Agent identity per project

To confine the agent's posts and pushes to one project, create a Project Access Token (GitLab) for that project, or a token for that repository only. Set `token_ref` and `agent_identity` together in the project's settings, and only that project's agent uses that token and the bot's identity. The inbox uses the forge's `inbox_token_ref` regardless of the override.

```yaml
projects:
  - ref: gl:YOUR_GROUP/switchboard-sandbox
    path: ~/repos/switchboard-sandbox-gl
    host: ubuntu-a
    token_ref: switchboard/gitlab-sandbox-bot
    agent_identity: project_N_bot_XXXX   # the bot user of the project access token
```

## Usage

Create the state labels (`agent:ready`, `agent:plan`, `agent:go`, `agent:running`, `agent:waiting`, `agent:done`) in the target repositories. Labels that already exist are left as they are.

```bash
switchboard labels sync
```

Run the agent on the host in charge of the projects. Ctrl-C stops it.

```bash
switchboard agent run
```

Put `agent:ready` on an issue, and on the next poll it becomes `agent:running` and `claude -p` runs in a worktree of its own for the issue. A run ends in one of two ways.

- With a question left: the agent posts the question as a comment and sets `agent:waiting`. Reply to the issue, and the next poll runs it again
- Finished: if the work passes `verify_command` and the output checks, the agent pushes the `switchboard/issue-<n>` branch, opens a draft PR, and sets `agent:done`

Work on an `agent:done` issue can go on until you close it. When the PR has a problem or a conflict, reply to the issue, and the next poll runs it again. The run adds commits to the same branch, and if a PR is open, the agent keeps using it instead of opening a new one. In that case, the done comment starts with "Pull request updated". Conflicts with the base are resolved through `git merge`, never through a rebase, and pushes stay without force.

During a run, the agent posts one progress comment and rewrites it every minute, adding one line each time. Each line holds the time, the elapsed time, the tools used and how often, and the first line of the latest `claude -p` text (up to 120 characters). Lines identical to the previous one are skipped. Tool inputs and outputs, such as command contents and the paths of files passed to tools, are left out. Still, a file name in the text of `claude -p` shows up in that first line. Each rewrite goes through the secret check and `comments.check_command`, and a line that seems to contain a secret is withheld. If someone deletes the progress comment, the agent posts it again with the next line. Runs that end within a minute get no progress comment. The agent deletes the progress comment after it posts the result comment, and after a run is cancelled. If the agent crashed and left one behind, the recovery after the restart deletes it.

A transient failure of a forge request does not leave an issue stuck. When the agent fails to write back a run's result, it tries again on every poll, with no limit on the attempts. Before each retry, it reads the issue again. If the issue was closed or someone changed its labels, the agent leaves the labels alone. It then posts only the result comment that is not yet posted, and sends no notification. An issue whose evaluation failed partway is evaluated again on the next poll. While anything is left to retry, the agent polls every `poll_interval`, even when it is connected to the relay.

From another terminal, check the agent's state.

```bash
switchboard agent status
```

A run waiting for write-back shows `write-back pending since <time>` in the `runs:` field, set apart from the lines of running runs.

`switchboard inbox` lists the waiting issues of every project, GitHub and GitLab together, in the order they started waiting. Below them, the "Running" section lists the `agent:running` issues with the host, the elapsed time, and the last line of the progress comment. Issues without a progress comment yet show `?`. At the bottom, the "Done (waiting for close)" section lists the open `agent:done` issues with the host, the time since done, and the PR/MR URL, oldest first. Closing an issue removes it from the list. Row numbers run on from the waiting rows to the done rows. With `--json`, the output is a JSON array, and the `state` of each element is one of `waiting`, `done`, `running`, or `queued`. `--forge github` or `--forge gitlab` limits the list to one forge. When one forge cannot be reached, the command lists what the other forge returned, then exits with a nonzero code.

```bash
switchboard inbox
```

Use a row number from the list to read a question in full and reply to it. With `--editor`, you write the reply in `$EDITOR`.

```bash
switchboard show 1
```

```bash
switchboard reply 1 "Go with option B."
```

Every command takes `--config` to name the config file. Without it, a command reads `~/.config/switchboard/config.yaml`.

### Planning first

You can have the agent ask its questions in issue comments before it implements anything, and implement only after you approve the plan. To start, you always put exactly one label on the issue.

- `agent:ready`: follows the project's `plan_first`. With `always`, the agent always starts with a plan; with `auto`, only when one is needed. Without the key, or with `never`, it implements right away. The `true` of earlier versions reads as `always`, and `false` as `never`
- `agent:plan`: starts with a plan
- `agent:go`: implements right away

For `agent:ready` with `plan_first: auto`, the first plan run decides whether to skip the plan stage. It skips only when the issue and the trusted comments settle every design decision and nothing is left to ask. When in doubt, it asks. If it skips, it comments the plan with this first line:

> The requirements are clear, so the agent skips the questions and the approval and implements this plan now.

Without waiting, it then moves on to the implementation run under the same start, and that run implements the plan in the comment. The comment keeps a record on the issue of what the agent set out to build. Replies after a skip go to implementation runs, as in a cycle started with `agent:go`. Only the first plan run of a cycle can skip. An issue that has asked a question once waits for plan approval as before.

A run in the plan stage changes no files. It may use only the read tools and `git status`, `git diff`, and `git log`. If the worktree has changes after the run ends, the agent discards them. The run neither pushes nor opens a PR. It asks about all the open decisions in one comment, each with a recommended answer. Once nothing is left to decide, it comments the plan. Both end in `agent:waiting`, with the wait reasons `plan-questions` and `plan`.

When the answer to one question changes the options of others, as a question about the scope or the direction does, the run asks that question alone in its round. A question whose recommended answer follows from the issue or the code is a check. The comment puts the checks last, under the heading "Going ahead as recommended". The web inbox shows each check on one line, which opens to the full question so you can choose again. If every question would be a check, the run asks none and comments the plan.

Start your reply with the approval word (`/ok` by default).

- Reply `/ok` to the questions to take the recommended answers and go on to the next round. To change only some questions, add to it, as in `/ok q2 is B`. Question numbers continue across rounds
- Reply `/ok` to the plan to approve it, and the next run starts implementing. Any other reply is a request to revise the plan, and the agent posts the plan again
- Only the first word of the first line of a trusted person's comment counts. `/okay`, or a comment edited by someone not trusted, does not approve

You can change the approval word in the config.

```yaml
plan:
  approve_word: /ok
projects:
  - ref: gh:YOUR_GITHUB_USER/switchboard
    plan_first: auto                  # always, auto, or never
```

You can give runs in the plan stage additional instructions of your own: one file for all projects and one per project. When both exist, the agent joins them, the one for all projects first. Implementation runs do not get them; they can get a separate `run.md`. The instructions are plain files under `instructions/` in the directory of the config file. For all projects, the file is `instructions/plan.md`. A GitHub project's instructions go in `instructions/gh/<owner>/<repo>/plan.md`, and a GitLab project's in `instructions/gl/<group>/<project>/plan.md`.

```bash
switchboard instructions edit                 # instructions for all projects
switchboard instructions edit gh:o/switchboard  # instructions for one project
```

You can also edit them on the Settings page of the web inbox. A save may be up to 32 KiB, and the web inbox rejects content that seems to contain a secret. Each save records the SHA-256 of the content in the audit log. The files are kept per host, and the web inbox changes only the files on its own host. If the agent runs on another host, edit the files there, or sync them along with the config file.

### The run sandbox

Shell commands in a run execute inside a sandbox of their own for each run. They can write only to the worktree and `$TMPDIR`, and they cannot connect to the network. Credential locations (`~/.ssh`, `~/.config/gh`, and so on) and switchboard's config and state directories are unreadable to them. Commands in the sandbox run without permission rules. Denied operations (`git push`, `git rebase`, `gh`, and so on) stay denied inside the sandbox too.

Only `git` and the project's whole verify command (`make verify` or similar) run outside the sandbox, and permission rules decide those two. Other commands of the same tool (`make deploy`, for example) run inside the sandbox.

A run does not use the Claude Code user's global settings (the sandbox, hooks, plugins, and permission rules in `~/.claude/settings.json`). It does load the project settings of the target repository.

Linux hosts need `bubblewrap` and `socat`. For each run, before it starts claude, the agent checks that bwrap can create namespaces. When it cannot, as inside another sandbox or a container, the agent does not start claude. It marks the run as an error and gives the reason in a comment.

### Extra permissions and instructions for implementation runs

You can give extra allowed tools and additional instructions of your own to the runs that write files. These are direct implementation, implementation after approval, and the runs that fix a CI failure, a push failure, or a conflict with the base. With these, unattended runs can use the skills and CLIs you use by hand. Plan runs get the extra allowed tools as well, so that they can start the app for the pictures of their questions (ADR-068); their instructions are in `plan.md`.

Write the permissions in the config file, as Claude Code permission rules. A skill with `allowed-tools` (as many harness skills have) cannot be called unless `Skill` is allowed. The agent joins the global `agent.allow_tools_extra` and each project's `allow_tools_extra`.

```yaml
agent:
  allow_tools_extra: [Skill]
projects:
  - ref: gh:YOUR_GITHUB_USER/switchboard
    allow_tools_extra: ["Bash(openspec *)"]
```

Commands in the sandbox run without rules, so you rarely need `Bash` entries such as `Bash(openspec *)`. A command that needs the network or writes outside the sandbox fails even with an entry. The tool name of each `allow_tools_extra` entry (`Skill`, or `Bash` for `Bash(openspec *)`) is also added to the tools the run gets. Runs that write files otherwise get only `Bash`, `Read`, `Edit`, `Write`, `Glob`, `Grep`, and `WebSearch`. Plan runs get `Bash`, `Read`, `Glob`, `Grep`, and `WebSearch`, and `Edit` and `Write` in a project with `plan_prototype`. A plan run never keeps an edit: an `allow_tools_extra` entry of `Write` is still denied there without `plan_prototype`.

When switchboard reads the config, three kinds of entries are errors. They are an empty entry, an unrestricted `Bash` or `Bash(*)`, and an entry that names an operation every run denies (`git push`, `git rebase`, `gh`, `WebFetch`). A broad entry such as `Bash(git *)` passes, but the denied operations stay denied. Changes to a project's `allow_tools_extra` take effect without a restart; `agent.allow_tools_extra` needs a restart. You can write permissions only in the config file. The web inbox cannot change them.

The instructions go in the same directory as the plan instructions, in a file named `run.md`. For all projects, the file is `instructions/run.md`. A GitHub project's goes in `instructions/gh/<owner>/<repo>/run.md`, and a GitLab project's in `instructions/gl/<group>/<project>/run.md`. Saves get the same checks and audit log entries as the plan instructions, and you can also edit the files on the Settings page of the web inbox.

Each time a run starts, the audit log records `run.started`. This record holds the run ID, the kind of run, the list of extra permissions, and the SHA-256 of each instruction file used. For a plan run, the extra permissions are empty and the hash is that of `plan.md`. Matching the hash against the `instructions.saved` lines tells you which saved version the run used. Instruction content itself never goes into the log.

```bash
switchboard instructions edit --run                   # implementation instructions for all projects
switchboard instructions edit --run gh:o/switchboard  # implementation instructions for one project
```

For an example of instructions in a project that uses OpenSpec, see [examples/openspec.md](examples/openspec.md) (Japanese).

### Resuming the session across plan rounds

In the plan stage, a reply to the questions, or a reply asking to revise the plan, starts the next plan run. The next run starts as a fork of the previous session if the previous plan run on this host posted its questions or plan less than `agent.resume_within` ago. `agent.resume_within` is 1 hour by default. It does not have to read again the code it read before, and much of its input is read from the cache.

If you changed the plan instructions, the approval word, the language, the model, or the effort since the previous run, the next run starts fresh instead. The same goes when the worktree's HEAD has moved. Runs after approval and fix runs always start fresh. With `agent.resume_within: 0s`, plan runs also start fresh every time.

```yaml
agent:
  resume_within: 30m
```

### Retention of run directories

The agent deletes the files of each run (`prompt.md`, `stream.jsonl`, and others under `runs/<run-id>/`) once `agent.retention` (720h by default) has passed since their last write. It checks at startup and once a day after that. Files of running runs, of runs waiting for write-back, and of the latest run of each issue are kept. `0s` turns deletion off.

```yaml
agent:
  retention: 168h
```

### Model and effort of runs

`agent.model` and `agent.effort` choose the model and effort of runs. `agent.plan` overrides them for plan runs, and `agent.fix` for fix runs (CI and push failures, conflicts with the base). When a value is empty, Claude Code's default applies. The model and effort in the user's global settings are not used. Valid efforts are `low`, `medium`, `high`, `xhigh`, and `max`; any other value is a config error. Both take effect on restart.

```yaml
agent:
  model: opus
  effort: medium
  fix: { effort: low }
```

### Instructions for implementation runs only, and screenshots

Fix runs get `run.md` too. Instructions meant only for direct implementation (`agent:ready`, `agent:go`) and implementation after approval go in `implement.md`. It sits in the same directory as `run.md`, and the file for all projects is `instructions/implement.md`. When both exist, the agent joins it after `run.md`. Plan runs and the runs that fix CI, a push, or a conflict with the base do not get it. Saves get the same checks and audit log entries as the other instructions. `run.started` records the hash of this file under the `implement` key.

```bash
switchboard instructions edit --implement gh:o/switchboard
```

The same two kinds of runs always get a convention for where to put images. Under the convention, the run puts each image to attach to the issue directly in `.switchboard/screenshots/` in the worktree. The agent adds this path to `.git/info/exclude` and empties the directory before a run starts. When a run ends as done, the agent adds one comment with only the images to the issue, after the done post. It attaches only `.png`, `.jpg`, `.jpeg`, and `.webp` files, up to 5 MB each and 10 in all. That comment names each file left out and the reason. If the run commits an image, the agent stops the push and waits, as for a failed check.

On GitHub, the agent attaches the images with `gh issue comment --attach` of the host's `gh` (v2.99.0 or later). It gives `gh` only the agent's token and does not use your `gh` login. For GitLab, the agent uploads them through the uploads API and lists them in a note. When it cannot attach them, the agent posts a comment with only the reason and leaves the done state as it is. Examples are a missing or old `gh`, and a token without the permission. `switchboard doctor` checks the version of `gh` when there are GitHub projects.

You write the steps for taking screenshots and the permissions for the commands they need (`allow_tools_extra`) yourself. See [examples/screenshots.md](examples/screenshots.md) (Japanese) for an example.

### Pictures in plan questions (GitLab)

On a GitLab project, a plan run can show a picture of the screen in a question: one for the question (the screen as it is now) and one for each option (the screen once that option is chosen). A question has either a picture or a Mermaid diagram. The run picks a picture when the question is about how a screen looks or changes, and a diagram for a flow or a state. GitHub projects keep to diagrams, as GitHub has no API that puts an image anywhere in a comment.

The run takes the pictures with `switchboard shot`, which drives Chrome's headless shell on the host. A full Chrome or Chromium makes a Unix socket on start, which a run's sandbox refuses, so it does not start there (V58). Set the browser in `agent.browser`, or put `chrome-headless-shell` on PATH. Without a browser, plan runs are not told about pictures. The headless shell that Playwright installs works:

```yaml
agent:
  browser: ~/.cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-linux64/chrome-headless-shell
```

`switchboard shot` opens only pages on `localhost`, `127.0.0.1`, and `[::1]`, and the page cannot load anything from elsewhere. It waits up to 30 seconds for the page to answer. Each command of a run has a network of its own, so the run starts the app, takes the pictures, and stops the app in one command: `APP > /dev/null 2>&1 & switchboard shot ...; kill %1`. A plan run cannot ask for permission, so Claude Code refuses a command with `$(...)`, backticks, `$!`, a `VAR=value` prefix, or JavaScript with parentheses and `>` on its command line; the run writes its JavaScript with the Write tool to a file in `.switchboard/plan-images/`, the one place a plan run may write, and passes it with `--script` (V58). Write in `plan.md` how to start the app and which port it uses, in that shape:

```markdown
To take pictures, run `pnpm --dir web/ui exec vite --port 5199 > /dev/null 2>&1 & <shot> --url http://localhost:5199/ ...; kill %1`.
```

You can also try it by hand:

```bash
switchboard shot --url http://localhost:5199/ --out shot.png --width 390 --box "#save|move it here"
```

To show the screen after an option, the run tries three ways in order and stops at the first that is enough: marks on the current screen (`--box`, `--arrow`), JavaScript that changes the page before the picture (`--script`), and, only in a project with `plan_prototype: true`, a change to the source in the worktree. The agent throws every change of a plan run away afterwards; with `plan_prototype` it does not say so in the comment.

A picture need not be of the app. For what a Mermaid diagram draws poorly, such as a layout, things side by side, or a before and after, the run may draw an HTML page in `.switchboard/plan-images/`, serve it on the loopback with `python3 -m http.server`, and take it the same way. On a screenshot, it explains with the labels of `--box` and `--arrow`, or with a short caption that a `--script` adds to the page.

The agent uploads up to 10 pictures a round, PNG, JPEG, or WebP of up to 5 MB each. A picture that cannot be attached leaves a line under its question and `plan.image_dropped` in the audit log. The web inbox draws the pictures of the latest questions in their cards, read through the inbox token; others are links.

### Running as a service

You can run the agent as a systemd service, so it keeps running while you are logged out. The service runs as your own OS user and restarts on its own when it ends abnormally. Tokens are stored as systemd credentials encrypted with the host's key. Only the agent tokens for this host's projects are stored, never the inbox token.

The installer reads the tokens from the Secret Service, so unlock the keyring first. It writes the path of the `switchboard` you ran into the unit. Use the one `make install` put in `~/.local/bin`. A binary built by `go run` lives in the build cache, where it can go stale or disappear, so the installer refuses it. First, check what the installer will run with sudo.

```bash
switchboard service install --dry-run
```

It shows the following commands, with one credential line per token. Tokens are passed on standard input and never shown on screen.

| Command | Purpose |
|---|---|
| `sudo install -d -m 0700 /etc/credstore.encrypted` | Create the directory for the credentials |
| `sudo systemd-creds encrypt --name=<name> - /etc/credstore.encrypted/switchboard.<name>` | Encrypt and store a token |
| `sudo install -m 0644 <temp file> /etc/systemd/system/switchboard-agent.service` | Install the unit |
| `sudo systemctl daemon-reload` | Load the unit |
| `sudo systemctl enable switchboard-agent.service` | Start it at boot |
| `sudo systemctl restart switchboard-agent.service` | Restart it (or start it, the first time) |

If that looks right, install it. The installer shows every command, then asks for confirmation with `[y/N]`. Type the sudo password in the terminal.

```bash
switchboard service install
```

Check the state with `switchboard service status` and the logs with `journalctl -u switchboard-agent.service`. Remove the service with `switchboard service uninstall`.

You can restart all the installed services (agent, relay, inbox, web inbox) at once. Services that are not installed are skipped. Type the sudo password in the terminal.

```bash
make restart
```

To restart on a fresh build, use `make deploy`. It builds the two binaries, puts them in `~/.local/bin`, then does what `make restart` does. `PREFIX` changes the install location. When the checkout is on a branch other than main, it warns. Run from a terminal, it asks whether to continue and stops on any answer other than `y`.

Run `switchboard service install` again in these cases:

- When you replace a token (to store the credentials again)
- When you move `claude` or `git` (the unit records the `PATH` from the time of installation)
- When you change the location of the config file or the projects this host is in charge of

switchboard has not checked whether a service may run with subscription authentication (`claude.auth: subscription`). Read Claude's terms of use and decide for yourself.

### Running on several hosts

The `host` of each project assigns it to a host. Even if a config mistake puts two hosts in charge of the same project, the two never run the same issue. When it takes up an issue, the agent posts a claim comment such as "Started on host ubuntu-a." and then reads the issue again. Only the host whose claim comment came first runs the issue; the others delete their own comments and back off. Both hosts must run this version of switchboard.

### Starting runs on replies right away with the relay

By default, the agent finds updates by polling, so a run can take up to `poll_interval` to start after you reply. With a relay (`switchboard relay serve`), the relay receives the forge's webhook and passes only the issue ID to the agent, which reads the issue again within seconds. The relay holds no issue bodies, comments, titles, or user names, and no forge token.

The relay in this version is not exposed to the internet. Use it for GitLab projects, with its endpoint set up in one of the two ways below. Webhooks from GitHub.com need a public endpoint, so switchboard handles GitHub projects by polling for now.

- GitLab runs on the same host as the relay: keep the default, and the endpoint listens on `127.0.0.1:8781`
- GitLab runs on another machine in the LAN: the endpoint listens on this host's LAN address. See [Receiving webhooks from GitLab on another machine](#receiving-webhooks-from-gitlab-on-another-machine) after the steps

While connected to the relay, the agent reconciles the projects whose webhooks are registered in `relay.hooks` every `reconcile_interval` (10 minutes by default). Projects without a webhook (GitHub projects, for example) get no events from the relay, so the agent polls them every `poll_interval` even while connected. If the relay stops, every project goes back to polling at `poll_interval` at once, and the agent reconnects when the relay comes back.

The setup steps are as follows.

1. Create a token for the host. The token is stored in the Secret Service, and the screen shows only the entries to add to the config (the token's hash and the agent's `relay`)

   ```bash
   switchboard relay token add <host>
   ```

2. In the GitLab admin area, open `Admin > Settings > Network > Outbound requests`. Add the endpoint's address and port (`127.0.0.1:8781` by default) to "Local IP addresses and domain names that hooks and integrations can access". Leave "Allow requests to the local network from webhooks and integrations" off. For a few minutes after you add it, GitLab's background jobs may refuse to send to this address
3. Register the project's webhook. The secret is stored in the Secret Service, and the webhook is registered with the inbox token. On GitLab, the webhook is registered with a signing token. If the token lacks the permission, the command shows the steps to register the webhook in the GitLab UI, once, along with the secret

   ```bash
   switchboard relay hook add gl:nas/switchboard-sandbox
   ```

4. Paste the entries shown in steps 1 and 3 into the `relay:` and `agent:` sections of the config file. switchboard does not rewrite the config file

   ```yaml
   relay:
     hooks:
       - project: gl:nas/switchboard-sandbox
         key: <hook-key>
         secret_ref: switchboard/relay-hook-<hook-key>
     hosts:
       - name: <host>
         token_hash: sha256:<hex>
   agent:
     reconcile_interval: 10m
     relay:
       url: http://127.0.0.1:8782/events
       token_ref: switchboard/relay-<host>
   ```

5. Install the relay as a service, and reinstall the agent in relay mode. The sudo commands have the same form as for the agent service, with the unit named `switchboard-relay.service`. Webhook secrets are the relay's only credentials

   ```bash
   switchboard service install --relay
   switchboard service install
   ```

Check the relay's state with `switchboard service status --relay` and its logs with `journalctl -u switchboard-relay.service`. To drop a webhook, run `switchboard relay hook remove <project>`, remove its entry from the config, and run `service install --relay` again.

GitLab temporarily disables a webhook that fails four times in a row, so a long relay outage may disable the webhook too. Reconciliation still handles the updates made while it is disabled. To enable the webhook again, send a test from `Settings > Webhooks` of the GitLab project.

On a GitLab that cannot use signing tokens, `relay hook add --gitlab-token` registers the webhook with `X-Gitlab-Token`. The config entry then gets `gitlab_token: true`. `X-Gitlab-Token` is a shared secret in plain text, so use signing tokens when you can. When the endpoint listens on a LAN address, you cannot use `--gitlab-token` or `gitlab_token: true`.

#### Receiving webhooks from GitLab on another machine

When GitLab runs on another machine in the LAN, the endpoint listens on this host's LAN address. The endpoint still speaks plain HTTP, so webhook payloads (issue bodies and comments) travel unencrypted inside the LAN. Do not expose it to the internet.

1. In the `relay:` section of the config, write this host's LAN IP address and port. Host names and all interfaces (`0.0.0.0`, `::`) are rejected as config errors. Without `webhook_url`, switchboard registers `http://<webhook_listen>` with GitLab

   ```yaml
   relay:
     webhook_listen: 192.168.0.82:8781
   ```

2. In this host's firewall, allow incoming connections to this port from the GitLab machine (for example, `sudo ufw allow from <GitLab address> to any port 8781 proto tcp`)
3. Add this address and port (for example, `192.168.0.82:8781`) to the allowlist from step 2 of the setup steps. Leave "Allow requests to the local network from webhooks and integrations" off
4. Register the webhook with a signing token. Signing tokens are available in GitLab 19.0 and later. To keep plain-text secrets off the LAN, switchboard refuses `relay hook add --gitlab-token`, and an entry with `gitlab_token: true` is a config error. Projects on a GitLab older than 19.0 get no webhook, and switchboard polls them instead

If GitLab's HTTPS certificate comes from a private CA, add that CA's certificate to the OS trust store of this host. Both switchboard's API calls and git's clone, fetch, and push read the OS trust store. This also covers the services that systemd runs.

```bash
sudo cp <private-ca>.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

### Telegram notifications for waiting and done issues

When an issue starts waiting or is done, the agent notifies you through a Telegram bot. A waiting notification carries the project, the issue number, the title, the wait reason, an excerpt of the question, and the issue URL. Done notifications carry the URL of the PR or MR the agent opened. The title and the question excerpt pass through Telegram's servers, so leave notifications off for projects where you do not want that.

1. Create a bot with @BotFather, send the bot a message, then look up the chat id. An existing bot works too
2. Store the bot's token in the Secret Service. `secret-tool` reads the token from standard input

   ```bash
   secret-tool store --label='switchboard telegram' service switchboard/telegram username token
   ```

3. Add a `notify` section to the config and reinstall the agent with `switchboard service install`. The bot's token also reaches the service as an encrypted credential

   ```yaml
   notify:
     telegram:
       chat_id: "<chat id>"
       token_ref: switchboard/telegram
     web_url: http://192.168.0.82:8783   # the web inbox; when set, notifications link to it
   ```

A failed notification does not stop the agent's work. The audit log (`~/.local/state/switchboard/audit.jsonl`) records the failure as `notify.failed`.

### The inbox in a browser

The web inbox (`switchboard-web`) shows the list of waiting and running issues and the thread of one issue, and you can reply from it.

- The header at the top of the page shows the counts of waiting and running issues, the state of live updates, and a "Settings" link
- At widths of 1024px and up, the list is on the left and the selected issue on the right. At narrower widths (phones), the list and an issue are separate pages
- An issue shows its body and comments, oldest first, rendered as Markdown. Raw HTML is shown as text, and HTML comments (`<!-- ... -->`) are not shown. Images are not loaded but shown as links. The agent's posts carry an "agent" tag. Claim comments take one line each, and only the latest progress comment is shown
- The running section shows the host, the elapsed time, and the latest progress line. A running issue takes no replies
- The queued section sits below the running section and shows the limit the issue hit and how long it has been queued. An issue that went from waiting for a reply to queued still takes replies
- The "Done (waiting for close)" section sits below the queued section and shows the host, the time since done, and a link to the PR/MR. The header shows the count of done issues only when there is at least one. You can reply to a done issue too, and a reply resumes the work on the same PR/MR. An issue leaves the section once you close it
- Done rows and the issue page have a "Merge" button. The confirmation dialog shows whether the PR/MR has conflicts, and the CI results: the counts of passed, failed, and running checks, with links to the failing and running ones. Both are information only and do not block the merge (ADR-050). Press "Merge" in the dialog, and the inbox token takes the PR/MR out of draft and merges it. The merge covers only the commit the agent reported as done; if commits were added to the PR/MR after that, the merge is refused. When the forge refuses the merge because of the CI results, a toast shows its reason. `Closes #N` in the PR/MR body closes the issue, which then leaves the list. When the base is not the default branch, the issue does not close, so close it by hand (ADR-043)
- If the merge is refused because the PR/MR conflicts with the base, the inbox service merges it automatically after the agent fixes the conflict. While it waits, the row and the issue page show an "Auto-merge pending" tag, and "Cancel auto-merge" on the issue page stops it. The automatic merge covers only the commits that the agent's fix runs (for conflicts, CI failures, and rejected pushes) pushed after you pressed the button. If someone comments or pushes to the PR/MR in the meantime, the automatic merge is called off. A toast shows the result the next time you open the page (ADR-048)
- A wait on a failed run (reason `error`) has a "Run again" button. Pressed while the reply field is empty, it posts a fixed reply asking to run again, and the agent does the same work over
- A wait that reached the limit of conflict fixes (reason `conflict`) also has "Run again". Pressing it posts a fixed reply asking to resolve the conflict, and the agent resets its count of conflict fixes and fixes the conflict again
- The list and the selected issue update on their own while the page is open. A reply you are writing survives the updates
- A project icon comes before each reference. By default, it is the forge's avatar: on GitLab, the project's avatar (or the group's, if the project has none), and on GitHub, the owner's. The inbox service fetches it with the inbox token and keeps it in `~/.local/state/switchboard/project-icons/` for 24 hours. When it cannot fetch one, the first letter of the project name is shown. An `icon` on a project in the config takes precedence (ADR-055)

  ```yaml
  projects:
    - ref: gh:YOUR_GITHUB_USER/switchboard-sandbox
      icon: "<one emoji>"                   # an emoji is shown as it is
    - ref: gl:YOUR_GROUP/tools
      icon: ~/icons/tools.png               # an image file on the host (starts with / or ~/)
    - ref: gh:YOUR_GITHUB_USER/notes
      icon: https://example.com/notes.png   # an image URL (https:// only; no token is sent)
  ```

  Images may be PNG, JPEG, GIF, or WebP, up to 256 KiB. SVG is not supported
- "New issue" above the list creates an issue in any project in the config. For the start label, choose one of the config's `labels.ready`, `labels.plan`, and `labels.go`, or "No label (do not start)". By default, ready is selected. The inbox token creates the issue, so with a start label, the agent takes it up as it is (ADR-040). Without a label, the agent leaves the issue alone until you add a start label later (ADR-049)

The work is split between two services.

- `switchboard inbox serve`: holds the inbox token. Over a Unix socket (`/run/switchboard/inbox.sock`), it serves the APIs for the list, one issue, and replies, plus SSE that announces changes to the list. It caches the list for 60 seconds. Only while a page is connected over SSE does it fetch the list from the forges again and announce changes. With push notifications configured (`inbox.notify.fcm`), it fetches the list even with no page open. The interval is `inbox.refresh_interval`: 30 seconds by default, and 10 seconds at least
- `switchboard-web serve`: calls that API and serves the pages on a LAN address. It relays the SSE to the browser and holds no forge token

While a page is visible, requests to the forges continue. With five waiting issues and a 30-second interval, the inbox service uses about 600 GitHub REST API calls and about 120 searches an hour. When the tab goes to the background, the page closes the SSE, and once no page is visible, the requests stop. To make fewer requests, lengthen the interval in the config file.

```yaml
inbox:
  refresh_interval: 60s
```

The web inbox is the only part that opens a listening port on a host (ADR-036). Expose it only inside the LAN. This version does not authenticate users, so any device inside the LAN can reply under your name and create issues that start runs (ADR-040). The `Origin` and `Host` checks refuse replies and issue creation from outside web pages. Reads such as the list are also refused with 421 when the `Host` is not this address, to prevent DNS rebinding.

1. Write the listen address in `~/.config/switchboard/web.yaml`, giving this host's LAN IP address and port explicitly. Only loopback, private (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`), and link-local addresses are allowed. `0.0.0.0`, `[::ffff:0.0.0.0]`, global addresses, and `100.64.0.0/10` (used by Tailscale and others) are rejected. Host names are not accepted, so that the `Host` check compares against this one value. Open the page in the browser by this IP address too

   ```yaml
   listen: 192.168.0.82:8783
   ```

2. Install the two services. The inbox service gets only each forge's `inbox_token_ref` as credentials. `switchboard-web` on the `PATH` runs as the web inbox service, which holds no credentials

   ```bash
   switchboard service install --inbox
   switchboard service install --web
   ```

3. Open `http://192.168.0.82:8783` in the browser. If your firewall closes the port, open it to the LAN only. With firewalld, for example:

   ```bash
   sudo firewall-cmd --permanent --zone=public --add-rich-rule='rule family="ipv4" source address="192.168.0.0/24" port port="8783" protocol="tcp" accept'
   sudo firewall-cmd --reload
   ```

Check the state with `switchboard service status --inbox` and `--web`. To try it by hand, start `switchboard inbox serve`, then `switchboard-web serve`. The socket is then created at `$XDG_RUNTIME_DIR/switchboard/inbox.sock`, and `socket` in `web.yaml` sets its location.

#### Logging in with GitLab

With `inbox.auth` in `config.yaml`, the web inbox asks whoever opens it to log in with GitLab (ADR-059). Only members of the GitLab groups you name (including their subgroups) and the one `owner` can log in. Without it, the web inbox asks for no login, as before.

1. Register an application in GitLab. As an administrator, create it under `Admin > Applications`; otherwise, under `Applications` in your user settings. Make it confidential and choose the scopes `openid`, `profile`, and `api`. Write the redirect URI from `listen` in `web.yaml`, as in `http://192.168.0.82:8783/auth/callback`
2. Store the secret that GitLab shows in the Secret Service. `secret-tool` reads the secret from standard input

   ```bash
   secret-tool store --label='switchboard gitlab oidc' service switchboard/gitlab-oidc username token
   ```

3. Add `inbox.auth` to the config and reinstall the inbox service with `switchboard service install --inbox`. The secret also reaches the service as an encrypted credential

   ```yaml
   inbox:
     auth:
       client_id: <application ID>
       client_secret_ref: switchboard/gitlab-oidc
       member_groups: [team]
       admin_groups: [team/leads]
       owner: <your GitLab username>
   ```

Login uses the GitLab at `forges.gitlab.base_url`. Sessions live only in the inbox service's memory, so everyone has to log in again when the inbox service restarts. A session ends after 24 hours without use, or 7 days after login. Anyone removed from the groups is locked out at the next token refresh (about every 2 hours). The header shows the logged-in username and "Log out".

The web inbox listens over HTTP for now, so the session cookie travels in plain text. Someone snooping on the same LAN can act as that user. Do not use it from outside your home, or on a LAN with devices you do not trust (#155).

In the Android app, the GitLab login page also opens inside the app.

#### Sharing the inbox with several people

With `inbox.auth`, several people can share one inbox (ADR-061). Replies, approvals, issue creation, and merges that the web inbox and the app send to GitLab run with the GitLab permissions of the person logged in. The forge records them as that person's actions. Writes by the agent use the permissions of the agent's token (the bot).

- The list and the issue page show only the GitLab projects that person can read
- GitHub projects are shown only to the `owner`. Writes to them use the GitHub inbox token
- Only people in `admin_groups` and the `owner` can edit the additional instructions and add projects. Everyone can open the Settings page. Others see only the "Display" section there, followed by "Only an admin can change the projects and additional instructions."
- Replies from people not in `trusted_authors` are posted, but they do not count as instructions to the agent. The page says so above the reply field
- An automatic merge runs under the identity of the person who pressed the button. Restarting the inbox service cancels it, and a toast says so the next time you open the page
- For each write from the web inbox, the audit log records a `web.action` line with the person who acted
- To approve from an Android notification's "Approve", open the app and approve on the page

Before setting it up, do the following.

1. Make the agent's identity a bot. Create a bot account or a Project Access Token, and write it in `forges.gitlab.agent_identity` and `token_ref` (or in the per-project override). Putting the agent's identity in `trusted_authors` is a config error
2. Make the GitLab inbox token (`forges.gitlab.inbox_token_ref`) one with the `read_api` scope. With login configured, switchboard uses this token only for reads. If it keeps the `api` scope, `switchboard doctor` warns
3. Put the people whose replies should count as instructions in `trusted_authors.gitlab`. Include the `owner`
4. Check with `switchboard doctor` that there are no warnings

#### Refreshing the list on relay events

On a host with a relay ([Starting runs on replies right away with the relay](#starting-runs-on-replies-right-away-with-the-relay)), the inbox service can connect to the relay too. When a webhook event arrives, it fetches the list again without waiting for `inbox.refresh_interval`. Events that arrive within 2 seconds of the first one are combined into one fetch, and no fetch happens within 10 seconds of the previous one. Changes in projects without webhooks (GitHub projects, for example) still reach the page through the requests every `inbox.refresh_interval`. Connecting to the relay does not lengthen this interval.

The inbox service stays connected to the relay for as long as it runs. When an event arrives with no page open, it discards the cached list and fetches it the next time a page opens.

1. Create a token for the inbox service. It is separate from the agent's token and is stored at `switchboard/relay-<host>-inbox`. The screen shows the entry to add to `relay.hosts` (named `<host>-inbox`) and the entry to add to `inbox.relay`

   ```bash
   switchboard relay token add --inbox <host>
   ```

2. Paste the entries shown into `relay.hosts` and the `inbox:` section of the config file

   ```yaml
   relay:
     hosts:
       - name: <host>-inbox
         token_hash: sha256:<hex>
   inbox:
     relay:
       url: http://127.0.0.1:8782/events
       token_ref: switchboard/relay-<host>-inbox
   ```

3. Reinstall the relay and the inbox service. This token also reaches the inbox service as an encrypted credential

   ```bash
   switchboard service install --relay
   switchboard service install --inbox
   ```

The live-update state in the header changes with the relay's state as follows. Connections to and disconnections from the relay also appear in the inbox service's log (`journalctl -u switchboard-inbox.service`).

| Shown | Meaning |
|---|---|
| Live updates on (instant) | Connected to the relay. Changes in projects with webhooks show within seconds |
| Live updates on (every 30s) | Disconnected from the relay. Changes show every `inbox.refresh_interval` (the number is that interval in seconds) |
| Live updates on | `inbox.relay` is not configured |

#### Adding projects from the Settings page

The "Projects" section of the Settings page lists this host's projects. Each one shows whether it is cloned and which state labels are missing. With `inbox.project_root` in `config.yaml`, you can add projects from this section.

```yaml
inbox:
  project_root: ~/repos               # the parent directory for the clones of added projects
```

You enter only the reference (`gh:owner/repo` or `gl:group/project`) and a choice under "Starting with agent:ready". Three choices are offered: "Start with a plan if needed", "Always start with a plan", and "Implement right away", which stand for the `auto`, `always`, and `never` of `plan_first`. By default, "Start with a plan if needed" is selected. The inbox service checks with the inbox token that it can read the repository and looks up the default branch. Then it appends one entry to `projects` in `config.yaml`, with `path` set to `<project_root>/<repository name>`, `host` to this host, and `base_branch` to the default branch. Comments and other entries stay as they are, and the original file is kept as `config.yaml.bak`. For other keys such as `verify_command`, edit `config.yaml` by hand.

The inbox service refuses to add a project in these cases:

- The same project is already in the config
- No project on this host uses that forge yet. Add the project to `config.yaml` by hand and run `switchboard service install` again, so that the agent gets that forge's token
- The inbox token cannot read the repository
- Another project is already cloned at the same path

When you add a GitLab project on a host that uses the relay, the page tells you to run `switchboard relay hook add <ref>`. Until you register the webhook, switchboard handles the project by polling. GitHub webhooks do not reach the relay, so the page shows no such hint for GitHub projects.

The agent and the inbox service check `config.yaml` every 10 seconds and reload only `projects`, `agent.max_parallel`, and `agent.prefer_plan`, with no restart needed. For a new project, the agent clones it with its own token and creates the state labels before it takes up the project's issues. If that fails, it tries again at the next reconciliation. When it cannot clone, check the agent's token (for a fine-grained PAT, the repositories it covers). Other changes still take effect on restart.

### Concurrent runs and the queue

The host-wide `agent.max_parallel` (1 by default) and each project's `max_parallel` cap the number of concurrent runs. A project without its own value is capped only by the host-wide limit; otherwise, the smaller of the two values applies. Issues of a project at its limit are queued, and issues of other projects run first. Lowering a limit does not stop the runs already going.

When a slot frees up, the agent starts the oldest queued issue. With `agent.prefer_plan: true` (false by default), it starts queued plan runs first and puts implementation runs after them. A plan run is a run in plan-first mode that posts questions or a plan. The reordering applies only when taking from the queue; if a slot is free, an implementation run starts right away too. When the agent restarts, the times issues entered the queue are lost, and the issues re-enter in the order they are evaluated again.

Fix runs start ahead of the queue: those for a done issue whose PR/MR conflicts with the base, and those for a failed CI. They come before plan runs even with `agent.prefer_plan` on. Among fix runs, conflicts are fixed first. Fix runs post no queued comment; the agent only logs that they started waiting for a slot.

```yaml
agent:
  max_parallel: 3
  prefer_plan: true   # take plan runs from the queue first
projects:
  - ref: gh:YOUR_GITHUB_USER/switchboard
    max_parallel: 1
```

When an issue is queued, the agent posts one comment on it that starts with "Queued:". Its text gives only whether the limit it hit is the host's or the project's, and the value of that limit. Labels stay as they are. The agent deletes this comment when the issue's turn comes and it starts, or when the start label is removed or the issue is closed. When the agent restarts, it deletes the comments the previous process posted, and posts them again for issues still queued. One log line marks an issue entering the queue, and another marks it starting from the queue.

### Android app and notifications

The Android app in `app/` opens the web inbox in a WebView. Its pages are the web inbox's own, so updating the web inbox updates the app too. On top of the web inbox, the app adds the following (ADR-047).

- It opens full screen from the home screen. Links outside the web inbox (forge pages, for example) open in the default browser
- Push notifications arrive for waiting and done issues. Tapping a notification opens that issue
- A notification of a plan waiting for approval has an "Approve" button, which replies with the approval word without opening the app. If the plan was posted again after the notification arrived, the app does not approve and tells you so
- A done notification has a "Merge" button, which opens the merge confirmation dialog in the app

The inbox service sends the notifications through FCM. Telegram notifications come from the agent, so if you set up both, you get both. A notification carries only the issue reference, the wait reason, and the title, never the text of the question. These pass through Google's servers. Away from your home Wi-Fi, notifications still arrive, but the app cannot open the web inbox.

1. Create a Firebase project and register an Android app in it with the package name `io.github.uskanda.switchboard`. Save `google-services.json` outside the repository
2. In the Firebase console, under "Service accounts", create a private key (JSON) and store it in the Secret Service. `secret-tool` reads the key from standard input

   ```bash
   secret-tool store --label='switchboard fcm' service switchboard/fcm username token < firebase-adminsdk.json
   ```

3. Add `inbox.notify.fcm` to the config and reinstall the inbox service with `switchboard service install --inbox`. The key also reaches the service as an encrypted credential. With this setting, the inbox service fetches the list every `inbox.refresh_interval` even with no page open

   ```yaml
   inbox:
     notify:
       fcm:
         project_id: <Firebase project ID>
         credentials_ref: switchboard/fcm
   ```

4. Create a signing key. Keep the key and its passwords outside the repository

   ```bash
   keytool -genkeypair -v -keystore ~/keys/switchboard-app.keystore -alias switchboard -keyalg RSA -keysize 2048 -validity 10000
   ```

5. Build the APK. You need JDK 17, Android SDK platform 37 (package `platforms;android-37.0`), build-tools 37.0.0, and NDK 27.1. Install any that are missing with `sdkmanager "platforms;android-37.0" "build-tools;37.0.0" "ndk;27.1.12297006"`. Set `ANDROID_HOME` and `JAVA_HOME`, and pass the locations of the key and `google-services.json` in environment variables. If a variable is missing, the build stops before Gradle runs. The APK is written to `bin/switchboard.apk`

   ```bash
   export SWITCHBOARD_APP_KEYSTORE=~/keys/switchboard-app.keystore SWITCHBOARD_APP_KEY_ALIAS=switchboard
   export SWITCHBOARD_APP_KEYSTORE_PASSWORD=... SWITCHBOARD_APP_KEY_PASSWORD=...
   export SWITCHBOARD_APP_GOOGLE_SERVICES=~/keys/google-services.json
   make app-apk
   ```

6. Install the APK on the phone, open it, and enter the URL of the web inbox. Use the IP address and port of `listen` in `web.yaml`, with `http://` in front (for example, `http://192.168.0.82:8783`). If you enter a host name instead, the `Host` check rejects replies and approvals. To change the URL later, tap "Address" at the top of the screen

The app sends notification token registrations and approvals to the web inbox with POST. Both go through the same `Origin` and `Host` checks as replies. As the `Origin`, the app sends the URL you entered.

### Display language

The web inbox shows Japanese when the browser's language is Japanese, and English otherwise. When a browser has several preferred languages, the web inbox goes down the list and uses the first one that is Japanese or English.

Under "Language" in the "Display" section of the Settings page, you can choose "Automatic", "日本語", or "English". "Automatic" follows the browser's language. The page switches to the chosen language without reloading. Each browser keeps its own choice, which is never sent to the server. On the login page, a button switches to the other language.

Everyone can open the Settings page. With `inbox.auth` configured, only people in `admin_groups` and the `owner` see the "Projects" section and the sections of additional instructions.

The Android app shows its own screens, its notification buttons, and the name of its notification channel in the device language. That is Japanese on a device set to Japanese, and English on any other. After you change the device language, the notification buttons switch the next time the app starts. The web inbox inside the app picks its language in the same way as in a browser.

The agent's issue comments and the Telegram notifications follow `comments.language` in the config (`en` or `ja`; `en` by default).

```yaml
comments:
  language: ja
```

Text that the web inbox posts to an issue is written in the language the page shows at that moment. This covers the answers to plan questions that "Send answers" posts, and the fixed reply that "Run again" posts. The agent reads replies in either language.

### Push and CI failures

After it opens a PR or MR and sets `agent:done`, the agent checks the CI of the commits it pushed every `poll_interval`. The interval stays the same while the agent is connected to the relay. When all CI has finished with no failures, it stops watching without writing anything. If any conclusion is `failure` or `timed_out`, it puts the issue back to `agent:running` and comments "The CI failed, so the agent fixes it (1/2)". Then it starts a fix run. The fix run gets the last 200 lines of the logs of the failed jobs (up to 3 jobs) as untrusted data. After the fix is pushed to the same PR, the agent watches the CI again.

An issue gets at most `agent.max_fix_attempts` fixes (2 by default). If the CI still fails after that many fixes, the issue waits with the reason `ci`. The comment carries the failed checks, a summary of each attempt, and the end of the logs. When a trusted person replies or the start label is put on again, the count starts over from 0. With 0, the agent fixes nothing automatically.

In these cases, the agent stops watching without writing anything:

- The PR is closed or merged
- Someone pushes other commits to the same branch
- No CI appears within 10 minutes of the push
- The CI has not finished 6 hours after the push
- The token cannot read the CI

When a push fails, git's output decides what happens next. A transient failure, such as a network error, is retried up to 3 times within the same run. If the issue branch on the remote has someone else's commits (non-fast-forward), the agent starts a fix run that merges that branch. Only `git merge` is allowed in that fix run, and the agent never force-pushes. A token without enough permission, or a rejection by a rule of the remote such as branch protection or a pre-receive hook, makes the issue wait with the reason `push`. The comment carries git's output and what a person needs to do.

```yaml
agent:
  max_fix_attempts: 2   # 0 turns automatic fixes off
```

### Usage limits and quotas

When a run hits a usage limit of the AI service, the agent puts the issue back to `agent:ready`. It posts one queued comment that says the run stopped at a limit and when it resumes. The issue does not wait, and the agent sends no notification. Until that time, the host starts no new run. Issues that would have started in the meantime also get a queued comment with the resume time. Runs already going are not stopped. Once the time passes, the agent resumes the stopped issues in the same cycle. For an implementation run, it tells the new run in the prompt that the previous run stopped at a limit, and has it continue from the changes in the worktree. If the service does not say when the limit lifts, the agent waits for `agent.limit_wait` (30 minutes by default). The pause is kept in `pause.json` in the state directory and survives an agent restart. When a fix run for CI or a push hits a limit, the issue waits with the reason `error`, and the run does not count toward the automatic fixes.

With `claude.auth: api_key`, the agent adds up the cost of each run (the `total_cost_usd` that claude reports) by day and by month in `cost.json`. Once the total reaches `quota.daily_usd` or `quota.monthly_usd`, the host starts no new run until midnight (host time) of the next day or month. In the meantime, issues are queued with the reason `quota`. The agent checks only before it starts a run and never stops a run partway. With `subscription`, it does not count costs, and if you write `quota` anyway, `switchboard doctor` warns.

```yaml
agent:
  limit_wait: 30m       # how long to wait after a limit whose end is unknown
quota:                  # used only with claude.auth: api_key; a key left out is off
  daily_usd: 20
  monthly_usd: 300
```
