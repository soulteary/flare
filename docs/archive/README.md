# Historical documentation archive

These records preserve tested candidates, decisions, failures and limitations.
They are not current installation instructions. Start with the
[documentation index](../README.md) for current stable/development version scope.
Original entry paths remain short compatibility links for external bookmarks.

## Published releases and candidates

- [0.6.5 published verification](releases/release-0.6.5-verification.md)
- [0.6.6 release notes](releases/release-0.6.6.md)
- [0.6.6 original candidate preflight](releases/release-0.6.6-preflight.md)
- [0.6.6 final acceptance](releases/release-0.6.6-final-preflight.md)
- [0.6.6 published verification](releases/release-0.6.6-verification.md)
- [0.6.7 release guide](../release-0.6.7.md) remains in the active documentation
  while it is the current stable release.

## Development history and feedback

- [First security implementation](security/login-private-access-implementation.md):
  historical scope; subsequent status is in [comprehensive acceptance](../requirements/login-private-access-acceptance.md).
- [Feedback disposition after 0.6.4](issue-triage.md): records delivery in 0.6.5,
  not a current feature/status matrix. Keyword shortcuts mentioned there were
  retired in 0.6.6; privacy enforcement changed only in later development.

## Measurements and evidence

Historical raw results remain under `metrics-reports/` to preserve source identity
and reproduction links. Use the [performance index](../performance.md) and
[current security evidence index](../requirements/README.md) to find them.
Do not rewrite old numbers, binary hashes, known gaps or candidate-time outcomes
when a later implementation changes. Temporary machine paths identify original
runs; regenerate disposable binaries with the linked tools rather than assuming
those paths are still available.
