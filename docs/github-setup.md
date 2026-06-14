# GitHub setup walkthrough

This guide wires Quad to a GitHub organization. GitHub is the familiar on-ramp
for instructors coming from GitHub Classroom: start here with zero new
infrastructure, then — when you're ready to own your data — move new courses to a
self-hosted Forgejo/Gitea instance with the same templates and grading specs (see
[migrating-github-to-forgejo.md](migrating-github-to-forgejo.md)).

> The GitHub adapter is already implemented — this is configuration only, no code.

---

## Prerequisites

- A GitHub **organization** you administer (e.g. `cs-dept`). Unlike Forgejo, Quad
  **cannot create a GitHub org via API** — it must already exist, and the GitHub
  App below must be installed on it.
- Permission to create and install a GitHub App on that org.
- Go 1.25+ on the machine running Quad.

---

## 1. Choose (or create) the organization

Pick the org that will own classroom repositories, e.g. `cs-dept`. This becomes a
classroom's `host_namespace`. If Quad is started against a missing org, provisioning
fails fast with a clear message ("organization not found — create it on GitHub and
install the App on it") rather than a deep API error.

---

## 2. Create and install a GitHub App

GitHub: **Settings → Developer settings → GitHub Apps → New GitHub App**.

Grant only these permissions:

| Permission | Level | Why |
|---|---|---|
| **Administration** (repository) | Read & write | Repo creation (`POST /repos/:tmpl/generate`) and `LockRepo`, which archives a repo via `PATCH /repos/:o/:r {"archived":true}`. |
| **Contents** (repository) | Read & write | Cloning and pushing to student repos; generating from a template. |
| **Metadata** (repository) | Read | Required by GitHub for any App installation. |
| **Webhooks** (repository) | Read & write | `EnsureWebhook` registers a push webhook per repo so grading can trigger on student pushes. |
| **Members** (organization) | Read | Resolve org membership when managing collaborators. |

Then:

1. **Install** the App on your org (`cs-dept`), all repositories (or selected).
2. Note the **App ID** (e.g. `123456`) and, from the installation URL or the
   installations API, the **Installation ID** (e.g. `987654`).
3. Generate a **private key** and save the downloaded `.pem` somewhere readable by
   the Quad process, e.g. `/etc/quad/github-app.pem`.

```sh
export QUAD_GITHUB_APP_ID=123456
export QUAD_GITHUB_INSTALLATION_ID=987654
export QUAD_GITHUB_PRIVATE_KEY_FILE=/etc/quad/github-app.pem
```

> **GitHub Enterprise Server**: also set `QUAD_GITHUB_BASE_URL=https://ghe.cs-dept.edu`
> so API and clone targets match your instance.

---

## 3. OAuth for student self-claim and operator login

A GitHub App has its own **client ID** and **client secret** (App settings →
"Generate a new client secret"). Quad uses them for the OAuth flow that lets a
student prove their GitHub username and lets operators log in.

```sh
export QUAD_GITHUB_CLIENT_ID=Iv1.0123456789abcdef
export QUAD_GITHUB_CLIENT_SECRET=abcdef0123456789abcdef0123456789abcdef01
export QUAD_OAUTH_REDIRECT_URL=https://quad.cs-dept.edu/auth/callback
```

Register that exact redirect URL ("Callback URL") in the App settings. A single
`/auth/callback` endpoint serves every host; the state parameter carries the host
so callbacks route correctly.

**Privacy**: the OAuth scope reads only the GitHub `login` (username) and numeric
`id`. No real name, email, or SIS id is requested or stored.

---

## 4. Start Quad

A complete operator environment (concrete values; operator `alice`):

```sh
export QUAD_GITHUB_APP_ID=123456
export QUAD_GITHUB_INSTALLATION_ID=987654
export QUAD_GITHUB_PRIVATE_KEY_FILE=/etc/quad/github-app.pem
export QUAD_GITHUB_CLIENT_ID=Iv1.0123456789abcdef
export QUAD_GITHUB_CLIENT_SECRET=abcdef0123456789abcdef0123456789abcdef01
export QUAD_OAUTH_REDIRECT_URL=https://quad.cs-dept.edu/auth/callback
export QUAD_OPERATOR_HOST=github
export QUAD_ADMIN_USERS=alice
export QUAD_COOKIE_SECURE=1            # behind HTTPS
# grading (optional, see step 8):
export QUAD_GRADER=container
export QUAD_GRADER_IMAGE=python:3.12
export QUAD_GIT_CLONE_TOKEN=ghp_yourPATforPrivateClones

go run ./cmd/quad
```

Startup summary (abridged):

```
store: sqlite /absolute/path/to/quad.db
adapters registered: github
identity resolvers: github  operator-host: github
grading: container runner (runtime=docker image="python:3.12")
quad control plane listening on :8080
```

---

## 5. Tiered validation

Work outward from the cheapest check:

1. **Operator login** — visit `https://quad.cs-dept.edu/auth/login`, authenticate as
   `alice`, then confirm:
   ```sh
   curl -s --cookie "quad_session=…" https://quad.cs-dept.edu/auth/me
   # {"username":"alice", ...}
   ```
2. **Classroom** — `host: github`, the org as `host_namespace`:
   ```sh
   curl -s -X POST https://quad.cs-dept.edu/classrooms \
     -H "Content-Type: application/json" \
     -d '{"name":"CS 101 — Fall 2026","host":"github","host_namespace":"cs-dept"}'
   ```
3. **Assignment** — first mark the template repo as a **Template** on GitHub
   (repo Settings → ✓ Template repository):
   ```sh
   curl -s -X POST https://quad.cs-dept.edu/classrooms/<classroom-id>/assignments \
     -H "Content-Type: application/json" \
     -d '{"title":"Homework 1","slug":"hw-1",
          "template":{"host":"github","namespace":"cs-dept","name":"hw1-template"}}'
   ```
4. **Roster** (optional, only with `join_policy: roster`) — add allowed usernames.
5. **Student claim** — open `https://quad.cs-dept.edu/assignments/<assignment-id>/accept`
   in a private window, authenticate as a student account, and confirm a repo
   `hw-1-<student>` appears under `cs-dept`.
6. **Grading** — trigger a run (step 8) and read back the score.

---

## 6. Configure grading (optional)

Grading runs untrusted student code on Quad's own sandboxed container runner — it
does not depend on GitHub Actions:

```sh
export QUAD_GRADER=container
export QUAD_GRADER_IMAGE=python:3.12          # default; a spec may override
export QUAD_GRADER_RUNTIME=docker             # or podman
export QUAD_GIT_CLONE_TOKEN=ghp_yourPATforPrivateClones
```

The clone token is delivered to git via `GIT_ASKPASS` — never embedded in the
clone URL or process arguments. Trigger grading for a classroom's submissions:

```sh
curl -s -X POST \
  https://quad.cs-dept.edu/classrooms/<classroom-id>/assignments/<assignment-id>/grade
# {"status":"grading","jobs_enqueued":N,"skipped_unprovisioned":M}
```

After accepting, students land on **`/me`**, where they see their repo link,
deadline, grading status, score, and per-test results; they return via
`/student/login`.

---

## 7. Webhooks (auto-regrade on push)

With a webhook configured, a student `git push` re-runs grading automatically and
their `/me` page updates live. Quad registers the webhook on each repo when
`QUAD_WEBHOOK_URL` is set, signing deliveries with the secret below.

```sh
# FULL receiver URL, used verbatim. It must include the /webhooks/github path and
# be reachable BY GITHUB. The secret must match on both sides.
export QUAD_WEBHOOK_URL=https://quad.cs-dept.edu/webhooks/github
export QUAD_GITHUB_WEBHOOK_SECRET=$(openssl rand -hex 32)
```

Restart Quad and confirm the startup summary shows the webhook URL and
`webhook secret [github]: set`.

> **Reachability gotcha.** Cloud GitHub calls `QUAD_WEBHOOK_URL` from the internet,
> so it **cannot reach `localhost`** or a private LAN address. For local testing,
> expose Quad through a tunnel (e.g. an SSH/ngrok-style tunnel) and use that public
> URL. In production, use your real public hostname.

---

## 8. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `organization not found — create it on GitHub and install the App on it` | The org in `host_namespace` doesn't exist or the App isn't installed on it. Quad cannot create GitHub orgs. |
| `resource not accessible by integration` | The App is missing a permission (see step 2) or wasn't re-installed after permissions changed. Bump permissions and accept the new request on the org. |
| Template generate returns 404 / 422 | The source repo isn't marked as a **Template** (repo Settings → ✓ Template repository), or the App lacks Contents write. |
| Clone fails during grading (auth) | `QUAD_GIT_CLONE_TOKEN` is unset or lacks access to the private repo. For GHES, also set `QUAD_GITHUB_BASE_URL`. |
| Operator login rejected | The username isn't in `QUAD_ADMIN_USERS`, or `QUAD_OPERATOR_HOST` isn't `github`. |
| `unknown host "github" — valid hosts: …` on classroom create | The GitHub adapter isn't configured — check `QUAD_GITHUB_APP_ID`/`INSTALLATION_ID`/`PRIVATE_KEY_FILE`. |
| Webhook delivery `401` | The signature didn't verify: the secret in the GitHub webhook config doesn't match `QUAD_GITHUB_WEBHOOK_SECRET`. Re-set both to the same value. |
| Webhook `204`, no grading | The push wasn't matched to a submission — wrong repo namespace/name, or it was a non-push delivery (e.g. GitHub's `ping`). Pushes to tracked student repos return `202`. |

---

## Next: own your data

Everything you authored here — template repos, `grading.json`, the workflow — is
host-neutral. When you're ready, point next term's classroom at a self-hosted
Forgejo or Gitea instance with no rewrite. See
[migrating-github-to-forgejo.md](migrating-github-to-forgejo.md).
