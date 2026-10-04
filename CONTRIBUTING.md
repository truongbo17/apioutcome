# Contributing

Bug reports with a minimal reproduction are welcome. For behavior changes, describe the response, log, and trace you expect; include a regression test. Keep the root module compatible with Go 1.22 and the Fiber v3 module compatible with Go 1.25. Run `go test ./...`, `go test -race ./...`, and `go vet ./...` at the root, plus `go test ./...` and `go vet ./...` in `fiberv3outcome`. Changes to public error semantics should update `SPEC.md` and the README. Changes that affect benchmark comparisons should update `BENCHMARKS.md` with the environment and commands used.
