# Contributing

Keep the source small, readable, and portable. Reusable Go code belongs in `engine`; the runnable application belongs in `frontend`; build helpers belong in `scripts`.

Do not submit original assets, extracted binaries, generated private caches, credentials, or copyrighted source. Evidence and handoff notes belong in the sibling private `Research` portal. Mark uncertain technical conclusions explicitly.

Before submitting changes, run `go test ./...`, `go vet ./...`, and `go run . --help` from `frontend`.
