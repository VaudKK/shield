# Shield — Secure Evidence & Trusted Disclosure

Shield is a privacy-first evidence preservation and controlled disclosure
platform for people who need to safely preserve sensitive evidence related
to abuse, violence, harassment, corruption, or other incidents.

> **Shield does not determine whether an incident occurred, prove
> authenticity, or make legal or investigative conclusions.** It helps
> people preserve evidence with integrity, understand it with AI
> assistance, protect personal information, and share it on their own
> terms.

## Status

Phases 1–8 are complete (foundation, auth/security, evidence upload,
content safety, OCR & AI, redaction, controlled disclosure, UI polish).
Phase 9 (hardening) and Phase 10 (demo) are next — see
[Roadmap](#roadmap).

**No account required.** Opening Shield goes straight to a landing page
with **Create Secure Vault** / **Recover Existing Vault** — a Vault ID +
recovery key stand in for an email/password. Email/password accounts
(`/login`, `/register`) still work but aren't linked from the default flow.
A "vault" is the same `users` row as an email account, just created a
different way (`email` and `vault_id` are mutually exclusive, enforced by
a DB constraint) — so every evidence/redaction/disclosure feature works
identically for both.

## What it does

1. Preserves original evidence, unmodified and uncompressed, with a SHA-256 integrity hash.
2. Screens uploaded images for sensitive content before anything else runs (self-hosted, never auto-deletes).
3. Extracts and understands evidence via OCR + AI (summary, timeline, gaps).
4. Detects PII and requires explicit accept/reject before anything is redacted.
5. Redacts for real — pixel-level black boxes on images, in-place content-stream redaction on PDFs, automated face blurring.
6. Builds user-curated Safe Disclosure Packages — nothing leaves the platform except what was explicitly reviewed and included.
7. Keeps a full, append-only audit trail per piece of evidence.

## Architecture

```mermaid
flowchart LR
    subgraph Client
        FE[React / Vite / TypeScript]
    end

    subgraph "Shield Backend (Go)"
        API[Go + Chi API]
        OCR[OCR: Tesseract + PDF text]
        PII[PII Detection: regex + AI]
    end

    NN[NudeNet — content safety]
    PDFR[PDF Redact — PyMuPDF]
    PG[(PostgreSQL)]
    S3[(S3-compatible storage)]
    OPENAI[(OpenAI API)]

    FE -- REST /api/v1 --> API
    API --> OCR --> PII
    API --> PG
    API --> S3
    API -- classify --> NN
    API -- redact --> PDFR
    API -- analyze --> OPENAI
```

Original evidence is never modified or compressed. Every protective action
(redaction, face blurring, disclosure export) produces a new, separate
derived file. NudeNet and the PDF redaction service are self-hosted Python
sidecars for the two things Go can't do natively; both are optional and
degrade gracefully (never fail the request) when unreachable.

## Tech stack

**Frontend:** React 19, Vite, TypeScript, Tailwind CSS, React Router,
TanStack Query, React Hook Form + Zod, lucide-react.

**Backend:** Go, chi router, pgx/PostgreSQL, AWS SDK v2 (S3-compatible
storage), gosseract (Tesseract via cgo), esimov/pigo (pure-Go face
detection), openai-go, embedded SQL migrations, structured logging.

**Self-hosted services:** `nudenet-service/` (Python, FastAPI, NudeNet/ONNX)
for content-safety classification; `pdf-redact-service/` (Python, FastAPI,
PyMuPDF) for real in-place PDF redaction.

## Local development

### Prerequisites

- Node.js 20+, Go 1.24+ (auto-fetched via `GOTOOLCHAIN=auto`)
- Docker (for Postgres, NudeNet, PDF redaction — or the full stack)
- Python 3.12+ only if running the Python services outside Docker
- An OpenAI API key (optional — analysis falls back to OCR + regex PII without one)

### Quick start

```bash
docker compose up --build
```

Starts Postgres (`localhost:5433`), NudeNet (`:8000`), PDF redaction
(`:8001`), and the Go API (`:8080`, migrations applied automatically).
Then, separately:

```bash
cd frontend && npm install && npm run dev   # http://localhost:5173
```

### Without Docker

```bash
cp backend/.env.example backend/.env   # set DATABASE_URL
cd backend && go run ./cmd/api
```

Image OCR needs a working cgo/Tesseract toolchain; without one, the build
falls back to a stub that keeps everything else working but errors if you
actually try to OCR an image (`internal/ocr/tesseract_nocgo.go`). The
Docker image always has the real backend. To run the Python services
locally instead of via Docker:

```bash
cd nudenet-service      # or pdf-redact-service
python -m venv .venv
.venv/Scripts/pip install -r requirements-dev.txt
.venv/Scripts/uvicorn app:app --reload --port 8000   # 8001 for pdf-redact-service
```

## Environment variables

### Backend (`backend/.env`, see `.env.example`)

| Variable | Description |
| --- | --- |
| `DATABASE_URL` | PostgreSQL connection string (required) |
| `CORS_ALLOWED_ORIGINS` | Comma-separated allowed origins |
| `S3_ENDPOINT` / `S3_REGION` / `S3_BUCKET` / `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | Object storage credentials |
| `OPENAI_API_KEY` | Backend-only. Empty disables AI analysis (OCR + regex PII still run) |
| `OPENAI_MODEL` | Defaults to `gpt-4o-mini` |
| `AI_VISION_ENABLED` | `true` to also send image evidence to OpenAI directly, alongside OCR text (Tesseract struggles with natural photos). Off by default — a privacy-boundary change, not just a quality tweak |
| `NUDENET_SERVICE_URL` | Empty disables content-safety classification (uploads stay `quarantined`) |
| `PDF_REDACT_SERVICE_URL` | Empty falls back to a redacted text transcript instead of an in-place redacted PDF |
| `SESSION_SECRET` | Signs session cookies |

### Frontend (`frontend/.env`)

| Variable | Description |
| --- | --- |
| `VITE_API_BASE_URL` | Defaults to `/api/v1` via the dev proxy |

## Database

Plain SQL migrations, embedded in the binary (`backend/internal/db/migrations/`),
applied automatically and in order on every startup, tracked in
`schema_migrations`. Add a new numbered `.sql` file and restart to migrate.

## Running tests

```bash
docker compose up -d postgres
cd backend
set -a && source .env && set +a
CGO_ENABLED=0 TEST_DATABASE_URL="postgres://shield:shield@localhost:5433/shield?sslmode=disable" go test ./...
```

Most tests are pure unit tests. Integration tests that need Postgres, S3,
or OpenAI credentials skip themselves (`t.Skip`) when those aren't
configured. `CGO_ENABLED=0` only matters without a working cgo/Tesseract
toolchain.

Each Python service has its own suite:

```bash
cd nudenet-service      # or pdf-redact-service
.venv/Scripts/pip install -r requirements-dev.txt
.venv/Scripts/python -m pytest
```

## API reference

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | — | Liveness/readiness check |
| `POST` | `/api/v1/vaults/` | — (rate-limited) | Create an anonymous vault; returns `{vault_id, recovery_key}` once, starts a session |
| `POST` | `/api/v1/vaults/recover` | — (rate-limited) | Recover a vault by ID + recovery key |
| `POST` | `/api/v1/auth/register` / `/login` | — (rate-limited) | *(Optional)* Email/password account |
| `GET` | `/api/v1/auth/me` | session | Current user |
| `POST` | `/api/v1/auth/logout` | session + CSRF | End session |
| `POST` | `/api/v1/evidence/` | session + CSRF, rate-limited | Upload evidence (multipart `file`, optional `title`) |
| `GET` | `/api/v1/evidence/` | session | List evidence |
| `GET` | `/api/v1/evidence/:id` | session | Evidence detail + signed URL |
| `DELETE` | `/api/v1/evidence/:id` | session + CSRF | Soft-delete |
| `GET` | `/api/v1/evidence/:id/moderation` | session | Latest content-safety scan (`404` if never scanned) |
| `POST` | `/api/v1/evidence/:id/analyze` | session + CSRF, rate-limited | Run OCR → PII → AI analysis (`409` if content-safety scan isn't done) |
| `GET` | `/api/v1/evidence/:id/analysis` | session | Summary, gaps, timeline, PII (`404` if not analyzed) |
| `GET` | `/api/v1/evidence/:id/pii` | session | Detected PII |
| `POST` | `/api/v1/evidence/:id/pii` | session + CSRF | Add a manual PII entry |
| `PATCH` | `/api/v1/evidence/:id/pii/:piiID` | session + CSRF | Accept/reject a PII item |
| `POST` | `/api/v1/evidence/:id/redact` | session + CSRF | Create a redacted copy of every accepted item |
| `GET` | `/api/v1/audit/:evidenceID` | session | Audit trail |
| `POST` | `/api/v1/disclosures/` | session + CSRF, rate-limited | Build a Safe Disclosure Package |
| `GET` | `/api/v1/disclosures/` / `/:id` | session | List / get a package |
| `GET` | `/api/v1/disclosures/:id/download` | session | Fresh signed URL for the package ZIP |

Errors use a consistent envelope: `{"error": {"code": "...", "message": "..."}}`.

## Security & privacy

- **Sessions:** server-side records in Postgres, referenced by an opaque
  random token (never the DB primary key) in an `HttpOnly`, `SameSite=Lax`
  cookie; only a hash of the token is persisted.
- **CSRF:** double-submit token pattern on state-changing requests.
- **Rate limiting:** per-IP token bucket on auth/vault/upload endpoints.
- **Uploads:** magic-byte type sniffing (never trusts client `Content-Type`),
  size-capped, streamed straight to private object storage, hashed from the
  same byte stream that's written (never trusted from the client).
- **Content safety:** every image is classified before being marked safe;
  a failure or borderline score degrades to `review`, never a silent
  `safe`. Nothing is ever auto-deleted for being flagged sensitive.
- **Access control:** every evidence/redaction/disclosure lookup is scoped
  to the requesting vault/user; a request for something you don't own
  returns the same 404 as a nonexistent ID.
- **Redaction:** the original file is never modified — every redaction is
  a new derived file. Only explicitly *accepted* PII is ever covered; a
  rejection is a durable guarantee, not just a UI state.
- **Vault recovery key:** shown exactly once at creation, hashed
  immediately (bcrypt), never logged or stored in plaintext, never
  accepted via URL parameters.
- Security headers set on every response; CORS restricted to an allowlist;
  secrets only ever read from environment variables.

## Roadmap

- [x] Phase 1 — Foundation
- [x] Phase 2 — Authentication & security
- [x] Phase 3 — Evidence upload
- [x] Phase 4 — Content safety (NudeNet)
- [x] Phase 5 — OCR & AI analysis (+ optional vision fallback)
- [x] Phase 6 — Redaction (pixel redaction on images, in-place on PDFs)
- [x] Phase 7 — Controlled disclosure (+ face blurring)
- [x] Phase 8 — UI/UX polish
- [ ] Phase 9 — Testing & hardening
- [x] Phase 10 — Hackathon demo

## Future improvements

Multi-factor authentication, organization/team accounts with granular
access control, offline-capable mobile capture, pluggable OCR/AI
providers, and a fuller accessibility and localization pass.
