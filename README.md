# QuickShortURL

**Less link. More possibility.**

A full-stack URL shortener built with **Next.js, Go, and MongoDB Atlas**. Create short links, choose custom endings, set expiration, and copy results in one click. Original destinations are encrypted before database storage.

Designed and developed by **Prathamesh Thakare**.

[Repository](https://github.com/thakare18/url-shortner) · [Screens](#frontend-screens) · [Local setup](#run-locally) · [Deployment](#deployment) · [Contact](#contact)

![QuickShortURL desktop homepage](docs/screenshots/home-desktop.png)

## Project overview

QuickShortURL turns long HTTP(S) URLs into compact, shareable links. The interface uses an industrial blueprint style: drafting grid, sharp borders, readable numbered fields, and one orange primary action.

Next.js presents the interface and proxies requests to Go. Go validates input, encrypts destinations, and saves records in MongoDB Atlas. Shared links work independently of the browser that created them. Frontend and backend have separate folders and deployment configurations.

**Status:** showcase/demo with working MongoDB integration. Hosting targets are **Vercel frontend + Render backend**. No live deployment URLs have been confirmed in this repository; example domains are illustrative.

## System architecture

```mermaid
flowchart LR
    User["User / browser"]
    Website["Original website"]
    subgraph Frontend["Next.js / Vercel"]
        UI["Form · preview · copy · policies"]
        Proxy["API and redirect rewrites"]
    end
    subgraph Backend["Go / Render"]
        API["HTTP handlers + validation"]
        Store["MongoDB storage layer"]
        Crypto["Encrypt / authenticate / decrypt"]
        Env["Backend environment: encryption key"]
    end
    subgraph Database["MongoDB Atlas"]
        Links[("links: encrypted destinations + metadata")]
        Codes[("link_codes: code hashes + expiry")]
        TTL["Background expiry cleanup"]
    end
    Action["GitHub Actions health request"]
    User --> UI
    UI --> Proxy
    Proxy --> API
    API --> Store
    Store <--> Crypto
    Env -. "key stays outside database" .-> Crypto
    Store <--> Links
    Store <--> Codes
    TTL -. "deletes expired records" .-> Links
    API -->|"302 redirect"| Website
    Action -. "GET /api/health" .-> API
```

Next.js forwards `/api/*` to Go and `/s/:id` to Go's `/redirect/:id`. Database credentials and encryption keys are never browser configuration.

Standalone SVGs: [architecture](docs/diagrams/architecture.svg), [creation workflow](docs/diagrams/create-link-workflow.svg), [redirect workflow](docs/diagrams/redirect-workflow.svg).

## End-to-end workflow

### Create and copy

```mermaid
sequenceDiagram
    actor User
    participant UI as Next.js interface
    participant API as Go API
    participant Crypto as Encryption
    participant DB as MongoDB Atlas
    User->>UI: Paste URL, optional alias and expiry
    opt Custom ending entered
        UI->>API: GET /api/availability?slug=...
        API->>DB: Check reserved code hash
        DB-->>API: Available / reserved
        API-->>UI: Availability feedback
    end
    User->>UI: Shorten URL or Enter
    UI->>API: POST /api/shorten
    API->>API: Validate and create metadata
    API->>Crypto: Encrypt original URL
    Crypto-->>API: Authenticated ciphertext
    API->>DB: Reserve unique code hash
    alt Code already reserved
        API-->>UI: Custom alias rejected (409) or generated code retried
    else Reservation succeeds
        API->>DB: Save encrypted link
        API-->>UI: 201 with result
        UI->>UI: Scroll to result and focus Copy
        User->>UI: Copy
        UI-->>User: Clipboard updated and Copied feedback shown
    end
```

Availability checks are debounced by 350 ms; stale requests are cancelled. The database decides uniqueness during creation. A preview does not reserve an ending. Failed saves return errors, not successful results.

### Open and expire

```mermaid
flowchart TD
    Open["Open /s/short-code"] --> Proxy["Next.js forwards to Go"]
    Proxy --> Lookup["Look up link"]
    Lookup -->|"Storage error"| Unavailable["503 · retry later"]
    Lookup -->|"Found"| Expiry{"Expired?"}
    Expiry -->|"No"| Decrypt["Authenticate and decrypt"]
    Decrypt -->|"Valid"| Redirect["302 · original website"]
    Decrypt -->|"Wrong key / tampered data"| Unavailable
    Expiry -->|"Yes"| Gone["410 · expired-link screen"]
    Lookup -->|"No record"| Reservation["Check code reservation"]
    Reservation -->|"Expired reservation"| Gone
    Reservation -->|"Unknown / incomplete"| Missing["404 · not found"]
    Gone --> Create["Create a new link"]
    TTL["Background TTL cleanup"] -. "removes expired destinations" .-> Lookup
```

Go enforces expiry immediately. Database cleanup can lag behind expiration. Minimal reservations remain so old codes cannot later lead to someone else's destination.

## Features

| Feature                | Behavior                                                                                 |
| ---------------------- | ---------------------------------------------------------------------------------------- |
| Clean URL input        | Large field, Enter submission and inline errors                                          |
| Custom endings         | Case-sensitive, 3–48 letters/numbers/hyphens/underscores; starts with a letter or number |
| Availability           | Debounced feedback and database-enforced uniqueness                                      |
| Full-link preview      | Same public base as the generated result                                                 |
| Expiration             | Never, 1 hour, 24 hours, 7 days or 30 days                                               |
| One-click copy         | Copied feedback for 2.5 seconds after a successful write                                 |
| Clipboard fallback     | Selects the URL for manual copying if clipboard access fails                             |
| Result navigation      | Scrolls to the result and focuses Copy                                                   |
| Clear / create another | Resets inputs/feedback and focuses the URL field                                         |
| Expired screen         | HTTP 410 page with a Create a new link action                                            |
| Responsive UI          | Desktop columns, mobile stacking and long-link wrapping                                  |
| Persistent storage     | Atlas records survive backend restarts/redeployments                                     |
| Privacy                | Encrypted destinations; no app visitor profiles or click analytics                       |
| Policies               | Privacy Policy and Terms of Service with developer contact                               |
| Metadata               | Titles, descriptions, canonical/social metadata, favicon, robots and sitemap             |
| Health checks          | Separate process liveness and database readiness                                         |

The backend accepts both HTTP and HTTPS **destinations**, although the UI label reads HTTPS SUPPORTED. Users must include the URL scheme; automatic HTTPS insertion is not implemented.

## Security algorithms and privacy

| Mechanism             | Purpose                                                                                  |
| --------------------- | ---------------------------------------------------------------------------------------- |
| AES-256-GCM           | Encrypts destinations using Go's standard cryptography packages                          |
| Random nonces         | NewGCMWithRandomNonce generates a fresh nonce and includes it in the ciphertext          |
| Authentication        | Rejects tampered data; binds destinations to record ID, short code and key ID            |
| Secure key generation | crypto/rand creates a random 32-byte key; Base64 is its configuration format             |
| Generated short codes | Six random bytes become 12 hex characters; collisions retry up to five times             |
| SHA-256 reservations  | Reserves code hashes without storing readable aliases in reservation records             |
| Unique indexes        | Prevents duplicate claims across backend instances                                       |
| Strict schema         | Rejects plaintext destination fields and invalid record structure                        |
| Input checks          | HTTP(S) only, no embedded credentials, URL/alias/expiry validation, bounded request body |
| Failure handling      | Storage/decryption errors fail closed instead of redirecting                             |

### What is stored?

`links` stores the internal record ID, unique code, **encrypted original URL**, encryption version/key ID, public short URL, custom/generated flag, creation date, expiration policy/date and schema version `2`.

Original URL, readable destination hostname and scheme are **not plaintext database fields**. They exist in memory for submission/redirect handling and in the creating user's response. The public short URL remains operational metadata.

Indexes: default `_id`, unique `short_code`, TTL `expires_at`, and descending `created_at`. Never-expiring records omit the TTL date.

`link_codes` retains only a SHA-256 code hash and optional expiry. Reservations remain indefinitely to prevent reassignment and preserve the expired page. Predictable aliases can still be guessed and hashed; hashing does not guarantee anonymity.

### Security boundaries

- Encryption protects database contents from readers without the key. The backend must decrypt valid destinations to redirect; someone with both database and backend-secret access can decrypt them.
- Anyone with an active short link can open it. Short links are not access control or destination-safety certification.
- The app does not collect IPs, browser/device profiles, locations, referrers or click analytics into link records. Hosting providers may keep separate connection/security logs.
- Keys stay outside MongoDB and browser configuration. Real .env files are ignored by Git.
- Copying, clearing a form or closing a tab does not delete a link. Expired destinations are deleted asynchronously; never-expiring destinations remain until removed. Backups can have separate retention rules.
- Redirects use Cache-Control: no-store and X-Robots-Tag: noindex, nofollow.

Back up the encryption key securely. Losing/replacing it makes existing destinations unreadable. Changing its ID alone does not rotate encryption. This version has one active key; rotation needs a separate re-encryption procedure. Go's random-nonce GCM API requires fewer than `2^32` messages encrypted per key.

## Frontend screens

Screenshots capture the actual UI with **illustrative links and mocked API responses**. They do not expose user data or claim a live deployment.

### Home — desktop and mobile

![Desktop shortening workspace](docs/screenshots/home-desktop.png)

<details>
<summary>Mobile homepage</summary>

<img src="docs/screenshots/home-mobile.png" alt="Mobile homepage with stacked inputs" width="390" />

</details>

### Custom ending, availability and preview

![Custom ending and complete link preview](docs/screenshots/custom-link-preview.png)

### Result, copy and create another

![Short URL ready with Copied feedback](docs/screenshots/short-link-result.png)

<details>
<summary>Mobile result</summary>

<img src="docs/screenshots/result-mobile.png" alt="Mobile result and copy button" width="390" />

</details>

### Expired link

![Expired page and create-new-link action](docs/screenshots/expired-link.png)

## Tech stack

| Layer           | Technology                                                 |
| --------------- | ---------------------------------------------------------- |
| Frontend        | Next.js 16.4.6 App Router, React 19.3.1, TypeScript 7.0.2  |
| Styling         | Responsive CSS; locally bundled Archivo and IBM Plex fonts |
| Backend         | Go 1.27.3, standard-library net/http                       |
| Database        | MongoDB Atlas; official MongoDB Go Driver v2.10.0          |
| Cryptography    | Go AES-256-GCM, SHA-256 and crypto/rand                    |
| Configuration   | Environment variables and godotenv                         |
| Hosting targets | Vercel frontend, Render backend                            |
| Automation      | GitHub Actions health schedule                             |
| Verification    | Go unit/integration tests; development browser checks      |

Versions are pinned in module/lock files. This frontend requires Node.js 20.9+.

## Usage examples

### Custom portfolio link

```text
Destination: https://example.com/portfolio/projects?source=quickshorturl
Custom ending: my-portfolio
Expiration: After 7 days

Example output: https://yourdomain.com/s/my-portfolio
```

### Generated link

```text
Destination: https://example.com/a/long/path
Custom ending: left blank
Expiration: Never expires

Example output: https://yourdomain.com/s/8f2c1ad0417b
```

```text
Local:      http://localhost:3000/s/my-portfolio
Production: https://YOUR-FRONTEND-DOMAIN/s/my-portfolio
```

A preview becomes active only after creation succeeds. Set PUBLIC_BASE_URL before creating production links. Changing it does not rewrite stored short URLs or turn existing localhost links into production links.

## Folder structure

```text
url-shortner/
├── .github/workflows/
│   └── render-health.yml          # Scheduled health request
├── backend/
│   ├── cmd/keygen/main.go         # Encryption-key generator
│   ├── .env.example               # Template; real .env ignored
│   ├── main.go                    # Handlers, validation, expiry, startup
│   ├── config.go                  # Settings validation and port binding
│   ├── store.go                   # MongoDB records, schema and indexes
│   ├── encryption.go              # Encryption/decryption and migration
│   ├── expired.html               # Expired page embedded in Go binary
│   ├── main_test.go               # Handler/configuration tests
│   ├── encryption_test.go         # Encryption and tamper tests
│   ├── store_integration_test.go  # Opt-in real database tests
│   ├── go.mod                     # Module and dependency versions
│   └── go.sum                     # Dependency checksums
├── frontend/
│   ├── app/
│   │   ├── components/site-footer.tsx
│   │   ├── legal/page.tsx         # Privacy and Terms page
│   │   ├── page.tsx               # Form, preview, result, copy and reset
│   │   ├── layout.tsx             # Fonts and metadata
│   │   ├── globals.css            # Responsive blueprint styling
│   │   ├── favicon.ico
│   │   ├── icon.png
│   │   ├── apple-icon.png
│   │   ├── robots.ts
│   │   └── sitemap.ts
│   ├── .env.example
│   ├── next.config.ts             # API/redirect rewrites to Go
│   ├── next-env.d.ts
│   ├── tsconfig.json
│   ├── package.json
│   └── package-lock.json
├── docs/
│   ├── diagrams/                 # Rendered architecture/workflow SVGs
│   └── screenshots/              # Repository-owned UI screenshots
├── .gitignore
├── render.yaml                    # Render service blueprint
└── README.md
```

No local MongoDB deployment or JSON storage folder is used. Storage uses the configured Atlas URI. In-memory storage exists only as a unit-test fake.

## API overview

The frontend proxies `/api/*` to Go. Public short links use `/s/:id`, forwarded to `/redirect/:id`.

| Method | Backend path                        | Purpose                              |
| ------ | ----------------------------------- | ------------------------------------ |
| POST   | /api/shorten                        | Create link; 201 on success          |
| POST   | /shorten                            | Legacy creation endpoint             |
| GET    | /api/availability?slug=my-portfolio | Available: true or false             |
| GET    | /api/config                         | Public base URL, no secrets          |
| GET    | /redirect/{id}                      | 302 active, 404 unknown, 410 expired |
| GET    | /api/health                         | Process liveness: status ok          |
| GET    | /api/ready                          | MongoDB readiness: status ready      |

### Create request

```http
POST /api/shorten
Content-Type: application/json
```

```json
{
  "url": "https://example.com/portfolio",
  "custom_alias": "my-portfolio",
  "expires_in": "7d"
}
```

Only url is required. Omitted/empty alias generates a code; omitted/empty expiration defaults to never.

Example 201 Created response:

```json
{
  "record_id": "691abc1234567890abcdef12",
  "id": "my-portfolio",
  "original_url": "https://example.com/portfolio",
  "short_url": "https://yourdomain.com/s/my-portfolio",
  "destination_host": "example.com",
  "destination_scheme": "https",
  "is_custom": true,
  "creation_date": "2026-11-17T10:00:00Z",
  "expires_at": "2026-11-24T10:00:00Z",
  "expiration_policy": "7d",
  "schema_version": 2
}
```

id is the short code; record_id is the database ID. This is the creating user's API response, not the encrypted database document. expires_at is null for never-expiring links.

PowerShell example:

```powershell
$body = '{"url":"https://example.com/portfolio","custom_alias":"my-portfolio","expires_in":"7d"}'
Invoke-RestMethod -Uri 'http://localhost:8080/api/shorten' -Method Post -ContentType 'application/json' -Body $body
```

| Status | Meaning                                                                     |
| ------ | --------------------------------------------------------------------------- |
| 400    | Invalid URL, alias, expiration or body                                      |
| 405    | Unsupported method                                                          |
| 409    | Custom ending already reserved                                              |
| 404    | Unknown/incomplete link                                                     |
| 410    | Expired link; branded HTML for redirects                                    |
| 503    | Storage unavailable, decryption failed or code-allocation retries exhausted |

Creation/availability errors return a JSON error message. Redirect errors may be text or the expired page. Rare secure-randomness failures return 500. There is no public list-of-links endpoint.

## Environment variables

### Backend

Use backend/.env locally and Render's Environment settings in production.

```env
MONGODB_URI=mongodb+srv://USERNAME:PASSWORD@CLUSTER.mongodb.net/?retryWrites=true&w=majority
PUBLIC_BASE_URL=http://localhost:3000/s
URL_ENCRYPTION_KEY=YOUR_BASE64_ENCODED_32_BYTE_RANDOM_KEY
```

These are placeholders. Never put actual secrets in documentation, screenshots or source code.

| Variable              | Requirement / default                                    | Purpose                                  |
| --------------------- | -------------------------------------------------------- | ---------------------------------------- |
| MONGODB_URI           | Required                                                 | Atlas connection string                  |
| URL_ENCRYPTION_KEY    | Required                                                 | Stable Base64-encoded random 32-byte key |
| PUBLIC_BASE_URL       | Local default http://localhost:3000/s; set in production | Public short-link base including /s      |
| MONGODB_DATABASE      | Optional; quickshorturl                                  | Database name                            |
| URL_ENCRYPTION_KEY_ID | Optional; v1                                             | Non-secret identifier for current key    |
| PORT                  | Optional; 8080 locally; Render supplies it               | Listening port                           |
| ADDR                  | Optional; 0.0.0.0:$PORT                                  | Override entire listening address        |

Hosting/process variables take precedence over .env. Run Go from backend so its file is found. Missing/invalid keys, database setup failures and unmigrated plaintext records prevent startup. There is no file-storage fallback.

### Frontend

Use frontend/.env.local or Vercel Environment Variables:

```env
BACKEND_URL=http://127.0.0.1:8080
# Set your real frontend origin before production build:
# SITE_URL=https://YOUR-FRONTEND-DOMAIN
```

| Variable    | Purpose                                                |
| ----------- | ------------------------------------------------------ |
| BACKEND_URL | Go origin without /s; default http://127.0.0.1:8080    |
| SITE_URL    | Frontend origin for canonical URLs/sitemap; without /s |

Set frontend variables before building and redeploy after changes. Without SITE_URL, canonical metadata is omitted and the sitemap is empty. With it, the homepage and /legal are listed. Search metadata does not guarantee rankings.

**MongoDB credentials and encryption keys must never be frontend or NEXT_PUBLIC\_ variables.**

## Run locally

### 1. Prerequisites and repository

Install Go 1.27.3 or a compatible newer toolchain, Node.js 20.9+, npm, and have a MongoDB Atlas cluster. No local MongoDB server is required.

```bash
git clone https://github.com/thakare18/url-shortner.git
cd url-shortner
```

### 2. Configure Atlas

Create a user with readWrite and dbAdmin scoped to quickshorturl. Startup creates/updates schema validators and indexes; collMod needs database administration permission. Cluster-wide administrator access is unnecessary.

Allow your development IP in Atlas Network Access. Copy the driver connection string, URL-encode special password characters, and keep TLS enabled.

### 3. Configure and start Go

For a **new installation**, from the repository root in PowerShell:

```powershell
cd backend
Copy-Item .env.example .env
go run ./cmd/keygen
```

Fill MONGODB_URI and save the generated output as URL_ENCRYPTION_KEY in .env. Keep a protected backup. For an existing installation, preserve its .env and key instead of overwriting/regenerating them.

```powershell
go run .
```

The default API is http://localhost:8080. Startup checks connectivity and prepares schema/indexes.

### 4. Start Next.js in another terminal

From the repository root:

```powershell
cd frontend
npm ci
npm run dev
```

Open [http://localhost:3000](http://localhost:3000). Use npm.cmd if PowerShell blocks npm.ps1.

### 5. Verify readiness

Open [http://localhost:3000/api/ready](http://localhost:3000/api/ready):

```json
{ "status": "ready" }
```

Create a link, copy it, and open it in another tab.

## Deployment

**Targets: Vercel frontend, Render backend, MongoDB Atlas storage.** Replace example domains with actual assigned origins. Deployment configuration is included; this README does not claim unverified live URLs.

### Vercel — frontend

Import the GitHub repository:

| Setting        | Value                             |
| -------------- | --------------------------------- |
| Root Directory | frontend                          |
| Framework      | Next.js                           |
| Build / output | Framework defaults                |
| BACKEND_URL    | https://YOUR-BACKEND.onrender.com |
| SITE_URL       | https://YOUR-FRONTEND-DOMAIN      |

An initial frontend deployment can establish the domain before Go exists. Shortening works after backend configuration. Redeploy Vercel after setting/changing its environment variables.

### Render — backend

Use render.yaml as a Blueprint or create a Web Service:

| Setting            | Value                                |
| ------------------ | ------------------------------------ |
| Root Directory     | backend                              |
| Runtime            | Go                                   |
| Build Command      | go build -o bin/server .             |
| Start Command      | ./bin/server                         |
| Instance Type      | Free, subject to provider limits     |
| Health Check Path  | /api/ready                           |
| MONGODB_URI        | Secret Atlas URI                     |
| URL_ENCRYPTION_KEY | Existing encryption key, as a secret |
| PUBLIC_BASE_URL    | https://YOUR-FRONTEND-DOMAIN/s       |

The Blueprint also supplies MONGODB_DATABASE=quickshorturl and URL_ENCRYPTION_KEY_ID=v1. Render supplies PORT; Go binds to 0.0.0.0:$PORT. Do not set ADDR to localhost on Render.

Open **Connect → Outbound** in the Render service and add its outbound IP ranges to Atlas Network Access. Keep permissions scoped to the application database.

Verify the deployment:

```text
https://YOUR-BACKEND.onrender.com/api/ready  → {"status":"ready"}
https://YOUR-FRONTEND-DOMAIN/api/ready       → {"status":"ready"}
https://YOUR-FRONTEND-DOMAIN/s/YOUR-CODE     → original website
```

Secrets belong on Render, not Vercel. PUBLIC_BASE_URL includes /s; BACKEND_URL and SITE_URL do not. Old localhost links do not automatically become production links.

Official guides: [Render Go](https://render.com/docs/deploy-go-nethttp), [Atlas connectivity](https://render.com/docs/connect-to-mongodb-atlas), [Vercel monorepos](https://vercel.com/docs/monorepos), [Vercel environment variables](https://vercel.com/docs/environment-variables).

### GitHub Actions health schedule

.github/workflows/render-health.yml requests /api/health every ten minutes, at minutes 3, 13, 23, 33, 43 and 53 (UTC). It checks health; it does not deploy the project.

1. Push it to the default branch and enable Actions.
2. Open **Settings → Secrets and variables → Actions → Variables**.
3. Add RENDER_BACKEND_URL=https://YOUR-BACKEND.onrender.com (origin only).
4. Test with **Actions → Render health check → Run workflow**.

Without that variable, the job skips. It uses bounded retries/timeouts and requires no database URI, encryption key or repository checkout. Its hostname check accepts Render onrender.com domains.

This is best effort. Scheduled runs can be delayed/dropped; inactive public-repository schedules can be disabled. Render free services can sleep/restart and have quotas. Pings do not guarantee zero cold starts. See [GitHub schedules](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule) and [Render free limits](https://render.com/docs/free).

## Testing and maintenance

```powershell
# From backend/
go test ./...
go vet ./...

# From frontend/
npm run typecheck
npm run build
```

Unit tests cover validation, metadata, redirects, expiry, concurrent claims, configuration, encryption round trips, unique nonces, wrong keys, tampering and swapped-record protection.

Real database tests are opt-in using a separate disposable test deployment/account:

```powershell
# From backend/
$env:MONGODB_TEST_URI='YOUR_DISPOSABLE_TEST_MONGODB_URI'
go test -count=1 -v ./...
```

Integration tests create/drop a unique quickshorturl_test_* database. They verify encrypted BSON, reconnect persistence, schemas/indexes, duplicate claims, expiry after cleanup and repeatable migration. **Never use the application's production account.** Without a test URI they skip. Normal runs simulate cleanup; optional MONGODB_TEST_TTL=1 additionally requires a dedicated test server with a fast TTL monitor.

Development browser checks covered copy, scrolling, previews, policy navigation and widths 320, 390, 768 and 1440 px. Screenshots use safe documentation fixtures. Browser scripts were development tooling, not a committed automated test suite.

Optional go test -race needs a compatible C toolchain. The development machine's 32-bit MinGW could not build Go's 64-bit race runtime. Normal tests, vet, builds and browser checks ran successfully during development.

### Legacy plaintext migration

Stop older backend instances, preserve the configured encryption key, and run from backend:

```powershell
go run . -migrate-plaintext
```

This encrypts legacy MongoDB destinations in place, removes readable destination fields, and preserves IDs, expiry and reservations. It is repeatable and exits after finishing. It does not rotate keys or erase provider backups.

## Contact

**Prathamesh Thakare** — developer of QuickShortURL.

| Channel  | Contact                                                                                 |
| -------- | --------------------------------------------------------------------------------------- |
| GitHub   | [github.com/thakare18](https://github.com/thakare18)                                    |
| Email    | [prathameshthakare9677@gmail.com](mailto:prathameshthakare9677@gmail.com)               |
| LinkedIn | [linkedin.com/in/prathameshv-thakare](https://www.linkedin.com/in/prathameshv-thakare/) |

Open to **hiring opportunities, freelance work, collaboration and project feedback**. For privacy questions, support or abuse reports, email the same address with the relevant short link and minimal necessary details. Do not send passwords, database credentials or encryption keys.
