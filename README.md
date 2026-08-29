# gerrit-checks-sdk-go

A **generated Go SDK** for the Gerrit **checks plugin** REST API (the `checksclient`
package), produced from the checks plugin's own statically generated **OpenAPI 3.1**
document. It is the plugin twin of [`gerrit-sdk-go`](https://github.com/davido/gerrit-sdk-go):
the same generate → XSSI → color pipeline, fed a **plugin** spec instead of gerrit-core's.
No hand-written request/response types — every operation and model comes from the spec.

## The pipeline (end to end)

```
  checks plugin                gerrit-checks-sdk-go       examples/
  (emit the spec)       -->    (this repo: the SDK)      -->   (consume the SDK)
  //plugins/checks:            openapi-generator (go)          go run, live calls,
  checks_openapi_json          + XSSI transport                colored output
```

1. **The checks plugin emits its own spec.** `bazel build //plugins/checks:checks_openapi_json`
   runs the core parse-only OpenAPI emitter over the plugin's `ApiModule` bindings —
   no running server, no reflection — enriched with prose mined from the plugin's own
   `resources/Documentation/rest-api*.md`. A build output, served live at
   `/plugins/checks/Documentation/rest-api-openapi.json`.
2. **This repo pins that spec** (`rest-api-openapi.json`).
3. **`generate.sh` generates the package** — openapi-generator (go) into `checksclient/`.
4. **A consumer reuses it by module path** — see [Use it](#use-it).

Demonstrates that the core OpenAPI initiative (Gerrit issue
[40011133](https://issues.gerritcodereview.com/issues/40011133)) works **per plugin**,
not just for gerrit-core.

## Version

Generated from **checks `3.15.0-SNAPSHOT`** (the plugin inherits Gerrit's version) and
tagged **`v3.15.0-SNAPSHOT`**. The Go module carries the `/v3` suffix (semantic import
versioning), exactly like `gerrit-sdk-go`.

## What's in this repo

- `checksclient/` — the generated package: **9 operations** (checks on a revision +
  `/plugins/checks/checkers` config) and **11 models** (`CheckInfo`, `CheckState`,
  `CheckerInfo`, …), over `net/http`, **stdlib-only** (no `go.sum`).
- `gerritxssi/` — the `)]}'` XSSI-stripping `http.RoundTripper`, **borrowed verbatim
  from `gerrit-sdk-go`** (the guard is identical on plugin endpoints, so nothing is
  plugin-specific here).
- `examples/list-checks/` — a runnable example: an anonymous
  `GET /changes/{id}/revisions/{rev}/checks` rendered as a colored, Web-UI-style summary,
  **green for successful / red for failed** checks, using Gerrit's own palette (borrowed
  from the `gerrit-sdk-go` example).
- `rest-api-openapi.json`, `generate.sh` — the pinned spec and the generation script.

## Regenerate

```bash
./generate.sh [path-or-url]      # default: ./rest-api-openapi.json
# straight from a running Gerrit that has the checks plugin installed:
./generate.sh https://gerrit-review.googlesource.com/plugins/checks/Documentation/rest-api-openapi.json
```

## The Gerrit-specific handling

The spec is consumed as-is; **no generated code is patched**:

- **XSSI guard** — every Gerrit JSON body starts with `)]}'`, not valid JSON and not
  OpenAPI-expressible. Stripped by the `gerritxssi` transport (`gerritxssi.Client()`),
  reused unchanged from `gerrit-sdk-go`.

The case-colliding `O`/`o` query params are handled by the go generator on its own
(distinct struct fields). Core's `--parameter-name-mappings r=regexFilter` is **not**
needed here — the checks API has no such collision.

**One emitter limitation to know:** the checks *list* endpoint is a synthesized
collection GET, so the emitted spec models its response as a skeleton `{type:object}` and
the generated `Execute()` returns an untyped map. The example therefore decodes the array
into the generated `CheckInfo` model directly (over the same XSSI transport). Single-check
`GET .../checks/{id}` is fully typed to `*CheckInfo`.

## Build & test

```bash
go build ./...        # build the SDK, transport, and example
go vet ./...          # static checks
```

### Test it live — no publish needed

The example lives *inside* this module, so its imports resolve to **local source**; Go
never hits a package registry. Run it against **upstream** (which runs the checks plugin)
right now — no auth, anonymous read:

```bash
go run ./examples/list-checks --change 623324            # a FAILED check (red)
go run ./examples/list-checks --change 621763            # 3 SUCCESSFUL checks (green)
go run ./examples/list-checks --change 623324 --no-color # plain text
go run ./examples/list-checks --change 623324 --revision 2   # a specific patch set
```

The output matches the change screen's **Checks** panel — e.g. change 623324 shows the
same red `Zuul Check Pipeline / Change failed` the Web UI does. (Pass flags directly; do
**not** prefix them with a bare `--`, which Go's `flag` treats as end-of-flags.)

### Test from GitHub — after publishing

Once pushed and tagged, the same import path is fetched from the VCS for any consumer:

```bash
go run github.com/davido/gerrit-checks-sdk-go/v3/examples/list-checks@v3.15.0-SNAPSHOT --change 623324
```

## Use it

```go
import (
	cc "github.com/davido/gerrit-checks-sdk-go/v3/checksclient"
	"github.com/davido/gerrit-checks-sdk-go/v3/gerritxssi"
)

cfg := cc.NewConfiguration()
cfg.Servers = cc.ServerConfigurations{{URL: "https://gerrit-review.googlesource.com"}}
cfg.HTTPClient = gerritxssi.Client() // strip the )]}' XSSI guard
client := cc.NewAPIClient(cfg)

// Fully-typed single check:
check, _, err := client.ChangesAPI.
	GetChangesChangeIdRevisionsRevisionIdChecksCheckId(ctx, "623324", "current", "zuul-check:gitiles").
	Execute()
```

```bash
go get github.com/davido/gerrit-checks-sdk-go/v3@v3.15.0-SNAPSHOT
```

Go modules publish by **git tag** — there is no artifact to upload; `go get` fetches the
tagged source directly from GitHub.

## Status

Prototype demonstrating that the Gerrit OpenAPI generator works **per plugin**
(issue [40011133](https://issues.gerritcodereview.com/issues/40011133)).

## License

Apache 2.0. See [LICENSE.txt](LICENSE.txt).
