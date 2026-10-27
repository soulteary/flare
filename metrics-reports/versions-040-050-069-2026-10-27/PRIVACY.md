# Published evidence redaction

Only the numerical experiment results and public software provenance are retained.

- Removed host/user directory names, host storage layout, local image/container identities,
  cgroup identifiers, precise wall-clock timestamps, cookie headers and test credential values.
- Replaced response CSRF values with equal-length `X` placeholders. Response byte counts and
  normalized hashes therefore retain their original meaning; `Hash` now authenticates the
  sanitized body. The original unredacted response bytes are not included.
- Replaced absolute sample timestamps with per-window relative offsets. Sequence and numerical
  CPU, RSS, latency, throughput, request counts and bootstrap calculations are unchanged.
- Archive owner/group names and IDs are empty/zero; archive timestamps and gzip metadata are
  normalized. Member manifests and archive hashes cover the sanitized published data.
- Collector directory configuration uses environment variables. Public Go module paths,
  frozen source/binary digests and standard container interfaces are software provenance,
  not host filesystem locations.

`verify.py` recomputes the complete original numerical summary from the sanitized records.
The published commit history is replaced to remove the earlier unredacted revisions from
this PR branch. Remote retention of unreachable commits is controlled by the hosting service.
