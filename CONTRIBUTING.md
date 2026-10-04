# Contributing

Bug reports with a minimal reproduction are welcome. For behavior changes, describe the response, log, and trace you expect; include a regression test. Keep the package compatible with Go 1.25 and run `go test ./...`, `go test -race ./...`, and `go vet ./...` before opening a pull request. Changes to public error semantics should update `SPEC.md` and the README.
