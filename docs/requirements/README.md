# Current security design and acceptance index

> Scope: single-administrator security work in current main development after
> stable 0.6.7. Implementation and candidate test evidence do not constitute a
> stable release or complete physical-device certification.

## User and operator guides

Start with [accounts](../accounts.md), [configuration](../configuration.md),
[upgrade/rollback](../login-private-upgrade.md), [backup/restore](../backup-restore.md)
and [proxy policy](../reverse-proxy.md). These are the operational instructions;
requirements and historical measurements below explain decisions and evidence.

## Requirements and architecture

- [Product requirements](login-private-access.md)
- [Phase-one implementation specification](login-private-access-phase1.md)
- [Security-storage ADR](security-storage-adr.md): transaction recovery, password,
  session, local-filesystem and single-writer boundaries

## Candidate acceptance

- [Comprehensive T01–T30 tracking](login-private-access-acceptance.md): authoritative
  candidate evidence index and remaining release/device conditions
- [Deployment, mounts and proxies](deployment-acceptance.md)
- [Migration, rollback and recovery](recovery-acceptance.md)
- [Scale, budgets and sustained load](scale-acceptance.md)
- [Desktop browsers, outbound traffic and editor/import](../../tests/security-browser/README.md)
- [Physical mobile checklist](../mobile-device-acceptance.md)

Each result applies to its recorded binary/source and environment. Subsequent
changes require relevant checks; historical passing results are not current-head
results. Hardware power loss and low-end ARM/NAS/mobile-device gaps stay explicit.

## Performance investigations

- [Administrator request proof reuse](admin-performance-optimization.md)
- [Administrator CPU/memory attribution](admin-resource-analysis.md)
- [Session transaction I/O consolidation](transaction-io-optimization.md)
- [Load-tool reproduction](../../tools/security-load/README.md)
- [Whole-project performance index](../performance.md)

These reports retain original before/after evidence, source differences and
limitations; they are not additional product requirements or permanent guarantees.
The [first implementation record](../archive/security/login-private-access-implementation.md) is
historical and superseded for acceptance status by the comprehensive tracking page.
