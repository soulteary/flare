# Feedback disposition after 0.6.4

This map covers work shipped in [0.6.5](https://github.com/soulteary/flare/releases/tag/0.6.5)
and explicitly separates remaining scope. It does not establish that every old
issue is fixed. Validate the original environment before closing a historical
report and link the actual release and verification evidence.

| docker-flare feedback | Disposition in 0.6.5 | Verification / remaining scope |
| --- | --- | --- |
| #172, #97, #76, #28 search engines | Configurable and hybrid search implemented | Presets/custom query encoding, keywords, local suggestions and browser acceptance |
| #173 category tabs | Tabs and native collapse implemented | Presentation filtering, not permissions or nested directories |
| #151, #143 columns | Desktop column limits implemented | Responsive maximums, not guaranteed count or masonry |
| #175 mobile grouping | Sequential and optional compact columns | Physical Safari/Via and native fonts remain unverified |
| #50, #65 readability | Greeting/navigation sizes and hidden descriptions | Device text scaling must remain enabled |
| #21 editor collection | List/forms/batch and recoverable versioned saves | Check individual original subcases; no automatic conflict merge or cross-file transaction |
| #183 privacy | Explicit valid private-access combination documented; parser hardened in 0.6.5 | Always verify anonymous access; encoded URLs and hidden settings are not authorization |
| #49 Kubernetes port injection | Typed environment parsing isolated per field | Numeric port / enableServiceLinks workaround documented; verify original cluster setup |
| #45 setting overwrite | Single-process section updates serialized | Same-section simultaneous edits and external writers are not auto-merged |
| #54, #133 subpath | Support boundary documented | Complete /flare/ mounting is not implemented; use a dedicated root hostname |
| #80 complex redirect | Dynamic/encoded source handling and Base64 plus round-trip fixed | Original target/browser fixture still useful; target-service policies are separate |
| #162 Raspberry Pi restart | Evidence checklist documented | Requires exact digest, platform, exit code and real-device run |
| #88, #37 favicon | Bundled favicon/cache behavior improved | User-configurable favicon remains a separate feature |
| #113, #81 icons | Built-in selector and cache improved | Local upload and a new brand-icon library remain separate features |
| #147, #67 internal/external URLs | Existing placeholders corrected | Two-address/manual-network switching is not implemented by these fixes |
| #186 browser capture | No feature delivery in this hardening batch | Authenticated capture API/bookmarklet/plugin remain separate work |
| #155 multi-user | No feature delivery in this hardening batch | Data isolation/permissions need a separate product/storage design |

For old user-resolved, duplicate or wrong-project reports, record the original
resolution and remaining subrequests instead of treating open/closed state alone
as delivery evidence. The [troubleshooting guide](troubleshooting.md) lists the
minimum evidence for current defects.
