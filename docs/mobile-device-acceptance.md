# Physical mobile acceptance

This checklist is a release gate for mobile/editor changes. Chromium touch
emulation is useful regression coverage; it does not certify physical Safari,
Via native font scaling, software keyboards, browser chrome or safe areas.
No physical device results are recorded by the automated suite.

## Record evidence

For each run record the exact commit/release and image digest (when using Docker),
device model, OS version, browser version, CSS viewport, orientation, native font
setting and any custom footer/CSS. Attach screenshots before/after the operation,
the actual result and a linked defect for every failure. Use a fresh browser
profile first, then repeat with the user's original configuration.

| Environment | Required settings | Current status |
| --- | --- | --- |
| Small iPhone / Safari | Portrait and landscape; software keyboard | Not verified |
| Safe-area iPhone / Safari | Browser bars expanded/collapsed; portrait/landscape | Not verified |
| Android / Chrome | Default and enlarged system text; software keyboard | Not verified |
| Android / Via | Native 110% and 125% font settings (issue #158) | Not verified |
| iPad / Safari | Split view; mobile and desktop user agents | Not verified |

Replace status only after a real run, with evidence. A desktop viewport with an
Apple user agent is not a physical Safari result.

## Fixtures

Use a disposable configuration containing at least: five categories, a group of
5 links and another of 30 links (issue #138), a long unbroken application name,
a long description, a long greeting, a long URL with query and fragment, Unicode
names, one custom-scheme/intranet link and a long footer. Keep a copy of both
YAML files before testing. Disable old custom layout overrides for the baseline,
then repeat affected cases with those overrides explicitly documented.

## Required operations

1. On home, applications, bookmarks and settings, confirm no page-level sideways
   scrolling. Confirm long text remains readable and links can be tapped. Check
   the title/date/weather at 767/768/769 CSS pixels or the closest split-view
   sizes the device permits. Single-column groups must preserve data order;
   double-column masonry is a separate optional feature, not this gate.
2. Set Via's native font size to 110%, then 125%; repeat the original #158
   navigation. Repeat Safari/Chrome with available OS/browser text enlargement.
   Changing `body.style.fontSize` is not an equivalent result.
3. Open New entry. Focus the last optional field with the software keyboard open.
   Scroll within the dialog, complete the form and save. Rotate while the dialog
   is open. The active field and Complete/Cancel actions must remain reachable.
4. Confirm the fixed save bar and last entry remain reachable with browser bars
   visible/hidden and the home indicator present. Trigger a long error message;
   buttons must wrap without covering all useful content.
5. Edit a long exact address and description; cancel, then repeat and complete.
   Only completed edits enter pending data; only Save changes persists them.
   Return navigation or reload must warn about a changed unfinished form.
6. Rename/reorder a category and explicitly migrate a nonempty category before
   deleting it. Save/reload and compare category IDs, placement and optional
   fields with the pre-test files.
7. Enter batch mode, swipe the table horizontally, edit an active cell, switch
   back, and save. Incomplete rows must block switch/save and remain editable.
   Delete all entries and add one again.
8. Disconnect the network before saving. Confirm all pending edits remain and
   retry after reconnection. Expire login in a separate window, retry and check
   that pending data is retained through reauthentication.
9. Open two editor windows. Save the first, then save the stale second window.
   Confirm conflict protection; export its draft, reload, restore and review
   before saving. Do not expect automatic merging.
10. If assistive technology is available, check labels, visible focus, modal
    focus containment and focus return. Record untested checks explicitly.

## Release decision

Block release for lost data, changed placement/fields, false save success,
unprotected conflicting saves, or inaccessible required actions. Cosmetic
preferences and unimplemented optional layouts stay in separate issues.
The release owner records PASS/FAIL/NOT TESTED for every environment; unavailable
hardware remains NOT TESTED with a follow-up owner, never inferred from Chromium.
