# upkeep-sample

A small Go notes API with token authentication and a real test suite. It is the demo target for [Upkeep](https://github.com/aalvaropc/upkeep):
- **Demo scenario:** Upkeep's demo builds nine pull requests on top of this code from recipes in Upkeep's repository, one blocker each, so they are not open here.
- **Real pull requests:** Upkeep also runs here as a GitHub Action on the real pull requests Dependabot opens.

## Run

You need Go 1.25 or newer.

```sh
export NOTES_JWT_SECRET="$(openssl rand -hex 32)"
go run ./cmd/notesd
```

In another terminal, with the same `NOTES_JWT_SECRET`:

```sh
token="$(go run ./cmd/notesd token -sub alice -scope notes:write)"
curl -s -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
  -d '{"title":"Groceries","body":"milk","tags":["home"]}' http://127.0.0.1:8081/v1/notes
curl -s -H "Authorization: Bearer $token" http://127.0.0.1:8081/v1/notes
```

| Variable | Default | Meaning |
|---|---|---|
| `NOTES_JWT_SECRET` | none | HS256 signing key, at least 32 bytes |
| `NOTES_ADDR` | `127.0.0.1:8081` | listen address |
| `NOTES_ISSUER` | `notesd` | token issuer |
| `NOTES_TOKEN_TTL` | `1h` | token lifetime |
| `NOTES_ALLOWED_ORIGINS` | none | origins allowed by CORS, separated by commas; none means no CORS headers |

## API

Every route except `/healthz` needs `Authorization: Bearer <token>`. Writes need the `notes:write` scope. Each user sees only their own notes.

| Method and path | Does |
|---|---|
| `GET /healthz` | health check |
| `GET /v1/notes?offset=&limit=` | list notes, oldest first (20 per page by default, 100 at most) |
| `POST /v1/notes` | create a note: `{"title", "body", "tags"}` |
| `GET /v1/notes/{id}` | read a note |
| `PUT /v1/notes/{id}` | replace a note's title, body and tags |
| `DELETE /v1/notes/{id}` | delete a note |

Errors are JSON: `{"error": {"code": "not_found", "message": "note not found"}}`. Every response carries an `X-Request-Id` header: the request's own ID when it sends a valid ULID, otherwise a new one.

## Development

`make check` runs gofmt, `go mod tidy`, `go vet`, the tests with the race detector, and the build. `make smoke` starts the server and calls it with a fresh token. CI runs both, plus secret scanning and a check that every pull request targets `main`.

| Workflow | What it does |
|---|---|
| `.github/workflows/upkeep.yml` | runs Upkeep every day, from the Actions tab, or on a `/upkeep` comment by a maintainer |
| `.github/dependabot.yml` | keeps the Go modules up to date |

## License

MIT. See [LICENSE](LICENSE).
