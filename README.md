# Shield — Secure Evidence & Trusted Disclosure

Shield is a privacy-first evidence preservation and trusted disclosure platform
for people who need to safely preserve sensitive evidence related to abuse,
violence, harassment, corruption, or other incidents.

> **Shield does not determine whether an incident occurred, prove authenticity,
> or make legal or investigative conclusions.** It helps people preserve
> evidence with integrity, understand it with AI assistance, protect personal
> information, and share it on their own terms.

## Status

This repository is being built in phases. **Phases 1–3 (Foundation,
Authentication & Security, Evidence Upload) are complete.** See
[Development approach](#development-approach) below for what's implemented
and what's next.

## Problem

People documenting abuse, harassment, corruption, or violence often have no
safe way to preserve evidence, understand what they have, or share it without
exposing themselves or others to further risk. Screenshots get compressed,
metadata leaks personal information, and there is no reliable record of what
was collected and when.

## Solution

Shield gives users a private workspace to:

1. Preserve original evidence, unmodified and uncompressed, with a verifiable integrity hash.
2. Understand and organize evidence with AI-assisted extraction, timelines, and summaries.
3. Detect personally identifiable information (PII) before anything is shared.
4. Review sensitive content and stay in control of how it's handled.
5. Build controlled, privacy-safe disclosure packages — deciding exactly what leaves the platform.
6. See a full, append-only audit trail of everything that happened to their evidence.

## Architecture

```mermaid
flowchart LR
    subgraph Client
        FE[React / Vite / TypeScript\nTailwind + shadcn/ui]
    end

    subgraph Shield Backend
        API[Go + Chi API]
        SEC[Security Gate\nauth, rate limit, validation]
        AI[AI Service\nOpenAI Responses API]
        OCR[OCR Service]
        PII[PII Detection]
    end

    NN[NudeNet\nPython sensitive-content service]
    PG[(PostgreSQL)]
    S3[(S3-compatible\nObject Storage)]
    OPENAI[(OpenAI API)]

    FE -- REST /api/v1 --> API
    API --> SEC
    SEC --> AI
    SEC --> OCR
    SEC --> PII
    API --> PG
    API --> S3
    API -- classification --> NN
    AI -- structured requests --> OPENAI
```

Original evidence is never modified or compressed. Derived artifacts (OCR
text, redacted copies, previews, disclosure exports) are always stored
separately from, and clearly linked to, the original.

## Tech stack

**Frontend:** React, Vite, TypeScript, Tailwind CSS, shadcn/ui, React Router,
TanStack Query, React Hook Form, Zod, Lucide icons.

**Backend:** Go, Chi router, PostgreSQL, embedded SQL migrations,
S3-compatible object storage, structured logging (`log/slog`), REST API,
environment-based configuration. Deployable to Railway.

**Planned (later phases):** NudeNet (self-hosted, Python) for sensitive-content
classification, OpenAI Responses API for evidence analysis.

## Security architecture

- **Authentication:** email/password with bcrypt (cost 12). Sessions are
  server-side records in PostgreSQL, referenced by an opaque random token
  (never the DB primary key) stored in an `HttpOnly`, `SameSite=Lax` cookie
  (`Secure` in production); only a SHA-256 hash of the token is persisted,
  so a database leak alone doesn't yield usable sessions. Login failures for
  unknown vs. known emails take the same code path and roughly the same time,
  so responses don't reveal which emails are registered.
- **CSRF:** a double-submit token pattern — a second, non-`HttpOnly` cookie
  whose value must be echoed in an `X-CSRF-Token` header on state-changing
  authenticated requests — backs `SameSite=Lax` as defense in depth.
- **Rate limiting:** a per-IP token-bucket limiter (`internal/ratelimit`,
  interface-based so a Redis-backed implementation can be swapped in for
  multi-instance deployments) guards `/auth/register` and `/auth/login`.
  This is a defense-in-depth layer, not a substitute for a DDoS/WAF layer
  such as Cloudflare in front of production — see the note in
  `internal/httpapi/middleware_ratelimit.go`.
- **Request validation:** JSON bodies are size-limited (1 MiB), decoded with
  unknown fields rejected, and validated field-by-field before touching the
  database.
- **Evidence upload:** auth → per-IP rate limiting → request size cap (55 MiB)
  → magic-byte file-type sniffing (never the client-supplied `Content-Type`)
  → streamed straight to private object storage. A malware/sensitive-content
  scan is the next stage in the pipeline (Phase 4); until then, every upload
  is recorded with `status: quarantined`.
- Original evidence files are stored privately under `originals/`; nothing is
  served via permanent public URLs. Reads only ever go through a signed URL
  with a 5-minute TTL, minted on demand.
- SHA-256 hashing on ingestion proves whether the *stored file* changed after
  upload — it is never presented as proof that the underlying evidence itself
  is authentic. The hash is computed from the same byte stream that is
  written to storage (not trusted from the client), via a `TeeReader`.
- Evidence lookups are always scoped to the requesting user; a request for
  evidence you don't own returns the same `404 EVIDENCE_NOT_FOUND` as a
  nonexistent ID, so the API never confirms or denies what other users have.
- Deletion is a soft delete (`deleted_at`), preserving the audit trail rather
  than destroying records.
- Structured logging never includes raw evidence content, passwords, session
  tokens, API keys, or sensitive PII.
- Secrets are only ever read from environment variables and are never
  committed to the repository.
- Security headers (`X-Content-Type-Options`, `X-Frame-Options`,
  `Referrer-Policy`) are set on every response; CORS is restricted to the
  configured origin allowlist with credentials enabled only for that
  allowlist.

## AI architecture

AI (OpenAI Responses API, Phase 5) is used only for organization, extraction,
summarization, and privacy protection — never to make investigative or legal
determinations. All AI output uses explicit JSON-schema structured outputs,
is validated server-side before it is persisted or acted on, and explicitly
represents unknown information as unknown rather than inventing it. The
OpenAI API key is a backend-only secret and is never exposed to the frontend.

## Local development

### Prerequisites

- Node.js 20+
- Go 1.24+ (the module targets a newer point release; `go build`/`go test`
  fetch that toolchain automatically on first use via `GOTOOLCHAIN=auto`)
- Docker (for PostgreSQL, and optionally the full stack)

### Quick start with Docker Compose

```bash
docker compose up --build
```

This starts PostgreSQL (on host port `5433`, to avoid clashing with a
locally installed Postgres on the default `5432` — the backend container
always talks to it internally at `postgres:5432`) and the Go API, which
applies migrations on boot, at `http://localhost:8080`. Then run the
frontend separately:

```bash
cd frontend
npm install
npm run dev
```

The frontend dev server runs at `http://localhost:5173` and proxies `/api` to
the backend.

### Running the backend without Docker

```bash
# Start Postgres however you like, then:
cp backend/.env.example backend/.env
# edit backend/.env with your DATABASE_URL

cd backend
go run ./cmd/api
```

The server applies pending SQL migrations automatically on startup.

### Running the frontend

```bash
cd frontend
cp .env.example .env   # optional; defaults to the dev proxy
npm install
npm run dev
```

## Environment variables

### Backend (`backend/.env`, see `backend/.env.example`)

| Variable | Description |
| --- | --- |
| `APP_ENV` | `development` or `production` |
| `PORT` | HTTP port (default `8080`) |
| `DATABASE_URL` | PostgreSQL connection string (required) |
| `CORS_ALLOWED_ORIGINS` | Comma-separated list of allowed origins |
| `S3_ENDPOINT` / `S3_REGION` / `S3_BUCKET` / `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | S3-compatible object storage credentials (used from Phase 3) |
| `OPENAI_API_KEY` | OpenAI API key, backend-only (used from Phase 5) |
| `SESSION_SECRET` | Secret used to sign session cookies (used from Phase 2) |

### Frontend (`frontend/.env`, see `frontend/.env.example`)

| Variable | Description |
| --- | --- |
| `VITE_API_BASE_URL` | Base URL for API requests. Defaults to `/api/v1` via the dev proxy. |

## Database setup

Migrations are plain SQL files embedded into the Go binary at
`backend/internal/db/migrations/`, named `NNNN_description.sql`. They are
applied automatically, in order, on every server startup, and recorded in a
`schema_migrations` table so each migration runs exactly once.

To add a migration, create a new numbered `.sql` file in that directory and
restart the server (or redeploy). So far: `0001_init.sql` (extensions,
`users`), `0002_sessions.sql` (server-side session store), `0003_evidence.sql`
(`evidence`, `evidence_files`, append-only `audit_events`).

## Running tests

```bash
cd backend
go test ./...
```

Most tests are pure unit tests and need nothing running. The auth flow and
CSRF/rate-limit integration tests run against a real Postgres when
`TEST_DATABASE_URL` is set; the evidence upload integration test additionally
needs real S3 credentials (`S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`,
`S3_SECRET_ACCESS_KEY`) and writes/reads/deletes under a throwaway key in
your actual bucket. All of these `t.Skip` when their prerequisite isn't set:

```bash
docker compose up -d postgres
cd backend
set -a && source .env && set +a   # loads S3_* and friends
TEST_DATABASE_URL="postgres://shield:shield@localhost:5433/shield?sslmode=disable" go test ./...
```

## S3 setup

Shield expects an S3-compatible bucket (Railway object storage, or any
S3-compatible provider) with logical prefixes:

```
originals/    # never modified after upload
processing/
previews/
redacted/
exports/
```

Only `originals/` is used so far. The S3 client (`internal/storage`) talks
path-style S3 with a configurable endpoint, so it works against Tigris,
MinIO, or AWS S3 itself — set `S3_ENDPOINT` to the provider's endpoint (or
leave it unset for AWS). The bucket must already exist; Shield never creates
or configures one.

## NudeNet setup

NudeNet runs as a separate, self-hosted Python service for sensitive-content
classification and is introduced in Phase 4. It is a classification aid, not
a legal, abuse, or CSAM detector, and Shield never automatically deletes
content it flags.

## OpenAI setup

Set `OPENAI_API_KEY` as a backend-only environment variable. The key is read
server-side only and is never sent to or exposed in the frontend bundle.
Integration lands in Phase 5.

## API documentation

Implemented so far:

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | — | Liveness/readiness check, including database connectivity |
| `POST` | `/api/v1/auth/register` | — (rate-limited) | Create an account, start a session |
| `POST` | `/api/v1/auth/login` | — (rate-limited) | Authenticate, start a session |
| `GET` | `/api/v1/auth/me` | session | Current user |
| `POST` | `/api/v1/auth/logout` | session + CSRF | End the current session |
| `POST` | `/api/v1/evidence/` | session + CSRF, rate-limited | Upload one file as new evidence (multipart: `file`, optional `title`) |
| `GET` | `/api/v1/evidence/` | session | List your evidence |
| `GET` | `/api/v1/evidence/:id` | session | Evidence detail, including a signed URL for the original |
| `DELETE` | `/api/v1/evidence/:id` | session + CSRF | Soft-delete evidence |
| `GET` | `/api/v1/audit/:evidenceID` | session | Append-only audit trail for one piece of evidence |

The full planned surface (timeline, PII, redaction, disclosures) is
documented as it's implemented in later phases; see the phase plan below for
the target shape.

Errors use a consistent envelope and never leak internal details:

```json
{
  "error": {
    "code": "EVIDENCE_NOT_FOUND",
    "message": "Evidence could not be found."
  }
}
```

## Privacy considerations

- 🔒 Original evidence is preserved privately and is never publicly accessible.
- 🔐 A redacted or exported copy never replaces the original.
- 👁 Users control what gets included in any disclosure package.
- Shield does not claim to determine whether an incident is true.

## Demo workflow

A guided, end-to-end demo (upload → secure → analyze → review → disclose →
audit trail) is built out in Phase 10, once the underlying features exist.

## Development approach

Built incrementally, in phases, each ending with tests, linting, a build
check, doc updates, and a commit before moving on.

- [x] **Phase 1 — Foundation:** repo structure, React/Vite frontend, Go
      backend, PostgreSQL connection, embedded SQL migrations, environment
      configuration, `/health` endpoint, basic dashboard shell, Docker
      Compose, README.
- [x] **Phase 2 — Authentication & security:** email/password auth with
      bcrypt, server-side sessions in PostgreSQL via `HttpOnly` cookies,
      double-submit CSRF protection, per-IP rate limiting on `/auth/*`,
      request validation, security headers. Tests cover the middleware and
      the full register → login → logout flow against a real database.
- [x] **Phase 3 — Evidence upload:** drag-and-drop upload UI, magic-byte file
      validation (JPEG/PNG/WEBP/PDF), SHA-256 hashing computed from the same
      stream written to S3-compatible storage, evidence + file metadata and
      an append-only audit trail (`EVIDENCE_UPLOADED`, `HASH_CREATED`,
      `EVIDENCE_VIEWED`, `EVIDENCE_DELETED`) in PostgreSQL, per-user
      isolation, soft delete, and short-lived (5 min) signed URLs for
      originals — never a permanent public link. Verified against the real
      configured S3 bucket (upload → presigned download → hash match) both
      in an automated integration test and by hand in the browser.
- [ ] Phase 4 — Content safety (NudeNet)
- [ ] Phase 5 — OCR & AI
- [ ] Phase 6 — Redaction
- [ ] Phase 7 — Controlled disclosure
- [ ] Phase 8 — Polish
- [ ] Phase 9 — Testing & hardening
- [ ] Phase 10 — Hackathon demo

## Future improvements

Beyond the phase plan above: multi-factor authentication, organization/team
accounts with granular access control, offline-capable mobile capture,
pluggable OCR/AI providers, and a fuller accessibility and localization pass.
