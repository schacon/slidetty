# Repository Guidelines

## Project Structure & Module Organization
The Go module `slidetty` lives at the repository root with `main.go` providing the Bubble Tea TUI entrypoint. Runtime assets stay in `slides/`; files beginning with an underscore configure metadata such as `_title.md`, `_author.md`, and `_theme.md`, while numbered `NN-topic.md` files control slide order. Use `examples/` as a reference deck when drafting new presentations, and keep generated binaries like `slidetty` out of version control.

## Build, Test, and Development Commands
Run `go run .` for a rapid feedback loop when iterating on the UI. Produce a distributable binary with `go build -o slidetty main.go`. Execute `go test ./...` before submitting changes—even if new packages introduce the first tests—to ensure the suite is green. `go fmt ./...` aligns the codebase with Go formatting prior to commit.

## Coding Style & Naming Conventions
All Go sources must remain `gofmt`-clean, which implies tabs for indentation and a single blank line between logical blocks. Follow idiomatic Go naming (`CamelCase` for exported identifiers, `lowerCamelCase` for internals) and keep functions focused so their intent stays obvious in the TUI update loop. Slide files should retain the zero-padded numeric prefix (`01-welcome.md`) to guarantee deterministic ordering, and shared helpers belong in future packages under this module rather than ad hoc scripts.

## Testing Guidelines
Use Go’s standard `testing` package with table-driven tests for slide loading, renderer helpers, or any new packages you add. House tests alongside sources in `_test.go` files and favor deterministic fixtures under `slides/` or a dedicated `testdata/` directory. Aim for meaningful coverage of parsing and navigation logic, and run `go test -cover ./...` locally when practical to catch gaps early.

## Commit & Pull Request Guidelines
Existing history favors concise, imperative commit subjects (for example, “add themes”); continue that pattern and limit body text to essential context. Each pull request should summarize the user-facing change, list manual or automated checks (e.g., `go test ./...`), and note any slide assets affected. Include screenshots or terminal recordings when UI output changes so reviewers can validate the new experience quickly.
