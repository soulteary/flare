# Flare 0.6.6 final release acceptance

Recorded 2026-10-26 (Asia/Shanghai). The release owner authorized fixing the
remaining icon compatibility and documentation issues, then publishing 0.6.6.
Publication and artifact checks are recorded in
[published verification](release-0.6.6-verification.md).

## Final result

Published stable 0.6.6 from `26596ab40ba9960bdcc8803719e211b45e2c26c8` through
[the successful tag workflow](https://github.com/soulteary/flare/actions/runs/40287730564).
All candidate and exact merged-main checks passed before tagging; regenerated
production assets left tracked source unchanged. Public seven-archive checks,
dual-registry four-platform metadata/anonymous pulls, actual published ARM64
containers, native icon/favicon HTTP and published-0.6.5 upgrade/rollback passed.
All-package race coverage is 86.4%. The physical-device exception below remains
explicitly disclosed. The list below records the acceptance requirements that
were applied; verification details distinguish execution from inspection.

## Changes since the previous preflight

Latest main includes YAML v3 unification, removal of unused legacy editor code,
MDI 7.4.47, Node.js 24 CI actions and themed favicons. The historical evidence in
[the original preflight](release-0.6.6-preflight.md) does not certify these changes.
The final candidate additionally restores all 19 retired MDI identifiers with
the original SVG paths shipped in 0.6.5. Upstream paths take precedence if a name
is restored in a later bundle. Compatibility also covers mixed-case names, inline
and generated-image navigation, previews and icon search; regeneration cannot
overwrite the compatibility file.

## Required final evidence

- Full all-package race/coverage suite at the release candidate; minimum 85%.
- Formatting, vet, lint, browser acceptance, CodeQL and security scan.
- Rebuild production assets without changing committed output.
- Seven local archives and four release Dockerfile platforms, including actual
  HTTP/timezone/restart acceptance under native execution or QEMU.
- A disposable published-0.6.5 upgrade, settings/editor writes, stale conflict
  rejection, restart and restored-complete-backup rollback.
- Numeric tag `0.6.6` must point to the verified final main commit. After tagging,
  verify the tag Release workflow, seven public archives/checksums, both registry
  manifests/four platforms, exact revision/version labels and public pulls.

## Physical-device exception

The release owner explicitly requested publication after resolving the two
remaining icon/documentation items, with the previously disclosed physical-device
limitation retained. Physical Safari/Via/Android/iPad and Raspberry Pi execution
remain NOT TESTED. This exception permits publication; it does not change any
device status to PASS. Follow-up owner: repository maintainer soulteary, to arrange
and record physical-device acceptance. No completion date is claimed.
