# The theme repository

> Availability: not in a release yet. The link on the theme page and the
> tool described here are in the source after 0.7.0.

Themes for Flare are listed in a repository of their own:
<https://github.com/soulteary/flare-themes>. It is separate from the source
of the application, so that a theme can be added or corrected without a
release.

Read this page for:

- [What the application does with it](#what-the-application-does-with-it):
  a link, and nothing else.
- [Installing a theme from it](#installing-a-theme-from-it).
- [What a theme has to meet](#what-a-theme-has-to-meet) to be listed.
- [The tool](#the-tool) that checks a theme and writes the index.
- [The index](#the-index): its format.
- [Proposing a theme](#proposing-a-theme).

## What the application does with it

The theme page links to the repository. That is all:

- The application never contacts the repository. It does not read the
  index, does not list or search themes, and does not look for newer
  versions of an installed theme.
- A theme gets onto an instance the way any theme does: it is put into the
  themes directory, [uploaded](theme-files.md#import-and-export), or
  [installed from an address](theme-files.md#installing-from-an-address)
  the administrator enters.
- An instance in offline mode shows the same link. Following it is done by
  the browser of the administrator, not by the instance.

## Installing a theme from it

1. Find the theme in the repository. Its entry in the index names its file,
   its size and its SHA-256.
2. Either download the file and upload it on the theme page, or enter the
   address of the file under **Install a theme from an address**. The
   repository says where its files are; for a file kept under `themes` it
   is an address of this form:

   ```text
   https://raw.githubusercontent.com/soulteary/flare-themes/main/themes/harbor.zip
   ```

3. The theme page shows the SHA-256 of what it fetched before anything is
   installed. Compare it with the one in the index.
4. A theme with templates is used with them only after templates are
   allowed for the site and those of the theme are confirmed; see
   [theme templates](theme-templates.md#turning-templates-on).

Being listed says that a theme met the rules below when it was added. The
administrator still decides what is installed: read
[what a stylesheet can do](theme-stylesheets.md) and, for a theme with
templates, [what the check does not cover](theme-templates.md#what-the-check-does-not-cover).

## What a theme has to meet

A theme of the repository is one file: `<name>.yml`, or `<name>.zip` for a
theme that is a directory. The tool checks the rules of the first table,
and a person checks those of the second.

| Rule | Checked by the tool as | What it means |
| --- | --- | --- |
| The application takes it in | `accepted` | The theme passes every check of an upload and is in use afterwards, with each of its templates accepted. This covers the contrast of its colors, a stylesheet that loads nothing from elsewhere, and the form of its assets |
| It is written for this format | `unused-part` | The theme file has no key, token or layout option the application does not use |
| An archive holds the theme and nothing else | `left-out` | No entry that an import would leave out, such as a preview image or a source file of a graphic |
| It is described | `described` | The theme file gives a `name`, a `version` and an `author` |
| It has a license that lets it be passed on | `license` | `license` is one of the SPDX identifiers `0BSD`, `Apache-2.0`, `BSD-2-Clause`, `BSD-3-Clause`, `CC-BY-4.0`, `CC0-1.0`, `ISC`, `MIT`, `Unlicense`, written exactly so |
| It is installed by itself | `accepted` | It stands on its own or is based on a bundled theme. A theme based on another theme of the repository would not be in use on an instance that has only the one |
| Its name is a theme name | `file-name` | Lower-case letters, digits and single hyphens, up to 32 characters, and not the name of a bundled theme |
| It is listed once | at the end of the report | No two files of the same name, as `harbor.yml` and `harbor.zip` |

| Rule | Why a person looks |
| --- | --- |
| It presents and does nothing else | A theme changes how the pages look. It does not hide the settings or the help, and it does not present content of its own as part of the site |
| Templates are read | A template can write any text and any link. Every template of a theme is read before the theme is listed, and again when it changes. The tool names the themes this holds for |
| The focus stays visible | The keyboard reaches every link and the search field, and it can be seen where the focus is |
| The name and the images are the author's to give | No name or image that belongs to someone else |
| The license is true | The author has the right to give the license the theme file names, also for fonts and images among the assets, and a text of the license travels with an archive as a [text document](theme-files.md#text-documents) |

A theme of tokens and a stylesheet can be listed on what the tool says and a
look at its pages. A theme with templates needs the reading.

## The tool

`tools/themecheck` reads a theme as the application of the same source
does, holds it against the first table, and writes the index.

```bash
go run ./tools/themecheck themes/harbor.zip themes/night.yml
```

```text
ok    harbor.zip (brings templates, to be read by a person: [home.html app.html bookmark.html group.html])
FAIL  night.yml: license: GPL-3.0-only
```

| Exit status | Meaning |
| --- | --- |
| `0` | Every theme meets the rules the tool checks |
| `1` | A theme does not, and the report names the file, the rule and what the rule is about |
| `2` | The tool could not do its work: a file cannot be read, or it was called without one |

- Each theme is checked by itself, in a temporary directory that is removed
  again. Nothing is written beside the files that are given.
- The tool makes no request to any host.
- A theme is checked against the release the tool is built from. A theme
  that names a later release under `requires` is not accepted by it.
- From outside this source the tool is run at a release:
  `go run github.com/soulteary/flare/tools/themecheck@<release>`.

With `-index` the tool writes the index to standard output, if every theme
meets the rules:

```bash
go run ./tools/themecheck -index themes/*.yml themes/*.zip > index.json
```

The index is the same for the same files, in whatever order they are given.
A repository can therefore write it again in its checks and compare it with
the one that is stored.

## The index

`index.json` lists the themes in the order of their names.

```json
{
  "format": 1,
  "themes": [
    {
      "name": "harbor",
      "file": "harbor.zip",
      "size": 2714,
      "sha256": "…",
      "title": "Harbor",
      "version": "1.0.0",
      "author": "The Flare documentation",
      "license": "MIT",
      "stylesheet": true,
      "assets": 0,
      "templates": ["home.html", "app.html", "bookmark.html", "group.html"]
    }
  ]
}
```

| Key | Value |
| --- | --- |
| `format` | `1`, the version of this format. It changes only when a reader of the format before could not read the index |
| `name` | The name the theme is installed under |
| `file` | The name of the file: `<name>.yml` or `<name>.zip` |
| `size` | The size of the file in bytes |
| `sha256` | The SHA-256 of the file in hexadecimal digits, which the theme page shows before it installs a theme from an address |
| `title`, `version`, `author`, `license` | What the theme file says; `title` is its `name` in English, or in the language it has |
| `requires` | The first release the theme works with, where the theme file names one |
| `extends` | The bundled theme the theme is based on, where it is based on one |
| `stylesheet` | Whether the theme brings a stylesheet |
| `assets` | The number of its images and fonts |
| `templates` | The names of its templates; an empty list for a theme without |

Where the files are kept and what else the repository holds beside them,
such as preview images, is for the repository to say. The index names
files, not addresses.

## Proposing a theme

How a theme is proposed is described in the repository. What can be done
before:

- Go through the list at the end of the
  [guide to writing templates](theme-template-guide.md#before-passing-a-theme-on),
  which holds for a theme without templates as well.
- Run the tool on the file and correct what it names.
- Download the theme from the instance it was made on as it is to be
  listed: a theme file as `<name>.yml`, a theme that is a directory as its
  [archive](theme-files.md#archives).

## Verification

- `internal/themerepo`: a theme that meets the rules, as a file and as an
  archive, with what the index lists for it; sixteen themes that each break
  one rule; several findings at once in a fixed order; a theme based on a
  theme that was checked before it; the same file checked twice; the order
  of the index and a name that is listed twice. The example of the
  [guide](theme-template-guide.md), packed as an archive, meets the rules.
- `tools/themecheck`: the report and the three exit statuses, the index in
  either order of the files, and that nothing is left beside the files.
- `internal/server`: the theme page has the link, opened in a new window
  without a referrer, also on an instance in offline mode.
- The repository itself, its checks and its files are not part of this
  source and are not covered here.
