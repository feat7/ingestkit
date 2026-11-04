# Contributing

Explain what a change does and how you checked it in the pull request.

1. Read [the delivery contract](docs/delivery.md) before changing ingestion behavior.
2. Reproduce a failure with a regression test before fixing its path.
3. Run `make test`, `go vet ./...`, `go build ./...` and `make test-integration`.
   For example or auditor changes, also run the Python/Node suites listed in
   [development](docs/development.md).
4. Describe the resulting behavior and verification, including limits that affect users.

Integration tests may create and remove only their own Compose projects. Keep
shared databases and other runs intact. Commit measured code before a timing
study, run studies sequentially, and keep unknown or limited outcomes in the
report. Performance claims need raw results and environment settings; weakening
durability changes the question. [Figure sources and checks](docs/figures/README.md)
cover data accuracy and visual review.

For documentation, explain the event or operator task before naming the internal
mechanism. Define unfamiliar terms where they first matter. Give each diagram a
question to answer, and keep measurement settings and unknown outcomes beside
the result. Code examples must still run after a prose edit.
