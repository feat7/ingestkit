# Contributing

Keep the collector small and its claims reproducible.

1. Read [the delivery contract](docs/delivery.md).
2. Add a behavioural regression test before changing a failure path.
3. Run `make test`, `go vet ./...`, `go build ./...` and `make test-integration`.
   Run the Python/Node suites when changing examples or artifact auditing; see
   [development](docs/development.md).
4. Explain the resulting behaviour and verification in the pull request.

Integration tests create and destroy only their own isolated Compose projects.
Do not reset a shared database, weaken durability for a benchmark, or add a
performance claim without its raw results and environment metadata. Keep timing studies sequential, commit the measured code first and
retain limited/unknown outcomes. Figure edits use [source/data and visual checks](docs/figures/README.md).
