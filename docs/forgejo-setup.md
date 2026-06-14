# Forgejo / Gitea setup walkthrough

This guide walks through wiring Quad to a self-hosted Forgejo (or Gitea)
instance, validated against a live Forgejo deployment. Gitea follows the same
steps — the `/api/v1` surface is identical.

---

## Prerequisites

- A running Forgejo instance (≥ 1.20 recommended)
- An admin account on that instance
- Go 1.25+ installed on the machine running Quad
- Outbound HTTPS access from Quad to the Forgejo instance

---

## 1. Create a Forgejo admin token

In Forgejo: **Settings → Applications → Access Tokens → Generate Token**

Required scopes (grant only these):

| Scope | Access | Why |
|---|---|---|
| Organization | Read & Write | `EnsureNamespace` creates the org on first use |
| Repository | Read & Write | Repo generation from template, collaborator management, archiving, webhooks |

Copy the token — you will not see it again.

```sh
export QUAD_FORGEJO_BASE_URL=https://forgejo.example.org
export QUAD_FORGEJO_TOKEN=fj_exampleTokenFromStep1
```

---

## 2. Create a template repository

1. Create a repo on your Forgejo instance (e.g. `instructor-org/hw1-template`).
2. Push your starter code.
3. Go to **Settings → Repository** and tick **✓ Template Repository**.

The generate endpoint returns an error if the source is not marked as a template.
Quad's adapter surfaces that error verbatim rather than silently failing.

---

## 3. Register an OAuth2 application (for student self-enrollment)

In Forgejo: **Settings → Applications → Manage OAuth2 Applications → Add OAuth2 Application**

| Field | Value |
|---|---|
| Application name | `Quad` |
| Redirect URI | `https://your-quad-host/auth/callback` |

Copy the **Client ID** and **Client Secret**.

```sh
export QUAD_FORGEJO_OAUTH_CLIENT_ID=2a1b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d
export QUAD_FORGEJO_OAUTH_CLIENT_SECRET=gto_exampleOAuthSecretFromStep3
export QUAD_OAUTH_REDIRECT_URL=https://your-quad-host/auth/callback
```

The same redirect URI serves all hosts — the state parameter carries the host so
callbacks are routed correctly regardless of whether a student authenticates via
GitHub or Forgejo.

---

## 4. Start Quad

```sh
# Minimal: token + base URL only (no student OAuth, no dashboard)
go run ./cmd/quad

# With student OAuth and admin allowlist:
export QUAD_ADMIN_USERS=alice,bob
export QUAD_OPERATOR_HOST=forgejo      # log operators in via Forgejo accounts
export QUAD_COOKIE_SECURE=1            # set behind HTTPS
QUAD_WEB_DIR=web/dist go run ./cmd/quad
```

On startup you should see (abridged):

```
store: sqlite /absolute/path/to/quad.db
adapters registered: forgejo
identity resolvers: forgejo  operator-host: forgejo
grading: DISABLED — grade requests will be rejected (set QUAD_GRADER=container)
dashboard: not mounted — set QUAD_WEB_DIR=web/dist (status page at /)
quad control plane listening on :8080
```

Verify with:

```sh
curl http://localhost:8080/healthz   # {"ok":true}
```

---

## 5. Create a classroom

```sh
curl -s -X POST http://localhost:8080/classrooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CS 101 — Spring 2026",
    "host": "forgejo",
    "host_namespace": "cs101-spring26"
  }' | jq .
```

`host_namespace` is the Forgejo organization slug. `EnsureNamespace` creates it
if it does not exist. The token must have Organization write access.

To restrict enrollment to students you pre-list, add `"join_policy": "roster"`:

```sh
curl -s -X POST http://localhost:8080/classrooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CS 101 — Spring 2026",
    "host": "forgejo",
    "host_namespace": "cs101-spring26",
    "join_policy": "roster"
  }' | jq .
```

---

## 6. Create an assignment

```sh
curl -s -X POST http://localhost:8080/classrooms/<classroom-id>/assignments \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Homework 1",
    "slug": "hw-1",
    "template": {
      "host": "forgejo",
      "namespace": "instructor-org",
      "name": "hw1-template"
    }
  }' | jq .
```

---

## 7. Student enrollment

Direct students to:

```
https://your-quad-host/assignments/<assignment-id>/accept
```

Quad redirects them to Forgejo's OAuth consent page. After approval, it:

1. Retrieves the student's Forgejo username and numeric user id (no real name,
   email, or SIS id).
2. Creates a roster entry (or finds the existing one).
3. Provisions `<student-username>-hw-1` under `cs101-spring26`.
4. Returns `{"status":"accepted","assignment":"hw-1","username":"<student>"}`.

If `join_policy` is `"roster"` and the student's username is not pre-listed, Quad
returns `403 {"error":"not on roster","username":"<student>"}` and enqueues nothing.

---

## 8. Configure grading (optional)

```sh
export QUAD_GRADER=container
export QUAD_GRADER_IMAGE=python:3.12       # default image; specs may override
export QUAD_GRADER_RUNTIME=docker          # or podman
export QUAD_FORGEJO_TOKEN=fj_exampleTokenFromStep1   # reused for private repo clones

# Clone auth: Forgejo token is delivered via GIT_ASKPASS — never in the URL.
# If your instance requires the token owner's username instead of "oauth2":
export QUAD_FORGEJO_GIT_USERNAME=alice
```

Trigger grading for all provisioned submissions in a classroom:

```sh
curl -s -X POST http://localhost:8080/classrooms/<classroom-id>/assignments/<assignment-id>/grade \
  | jq .
# {"status":"grading","jobs_enqueued":N,"skipped_unprovisioned":M}
```

---

## 9. Known Forgejo / Gitea quirks

| Quirk | Details |
|---|---|
| **422 on generate** | Forgejo/Gitea returns 422 for two distinct cases: genuine validation errors *and* "repo already exists". Quad disambiguates with a re-GET before treating 422 as failure. |
| **Template flag required** | The source repo must have the Template flag set. The adapter surfaces the raw 422 body (`{"message":"..."}`) so the error is actionable. |
| **`IncludeAllBranches`** | Gitea's generate endpoint copies only the default branch. `CreateRepoOptions.IncludeAllBranches` is accepted but silently ignored — a known upstream limitation. |
| **Org already exists** | `EnsureNamespace` treats a 422 with an "already exists" body as success (idempotent). |
| **HTTPS clone username** | `oauth2` is the widely-supported convention for Forgejo HTTPS token auth. If clones fail, try `QUAD_FORGEJO_GIT_USERNAME=<token-owner>`. |
