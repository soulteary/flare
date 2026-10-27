# Report wrappers

The `go-test-report` execution scripts and interface are adapted from
`soulteary/go-test-report-action` commit
`bf04b539f7bf2a6f67d602da4259904b3dcdeba1` (v1.1.2).
The `go-report-card` scripts and interface are adapted from
`soulteary/goreportcard-action` commit
`fc43049a5fe61c3313294d217c2a783f375a7970` (v1.1.3).
Both are licensed under Apache-2.0; see LICENSE.

Local changes remove nested setup-go/cache restores and source-build fallbacks.
GTR path validation runs after installation using the same pinned release CLI,
instead of compiling a second validation binary from Action sources.
The calling workflow supplies the project Go toolchain and owns its cache.
Only the same pinned release CLIs are installed, with mandatory SHA256 checks.
Report generation, result enforcement and writeback scripts remain upstream
implementations. Path validation still delegates to GTR's validate-paths command.
Review upstream changes explicitly when upgrading.
