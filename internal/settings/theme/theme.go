package theme

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/formvalue"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/pool"
	"github.com/soulteary/flare/internal/settings"
)

// actionSaveCustom is submitted by the custom color form and actionSaveMode
// by the form for light and dark. Theme cards submit only a theme name.
const (
	actionSaveCustom = "save-custom"
	actionSaveMode   = "save-mode"
	// actionImport is submitted by the form that uploads a theme file, in
	// the field importField. The box importReplace allows replacing a theme
	// file of the same name.
	actionImport  = "import"
	importField   = "theme-file"
	importReplace = "replace"
	// actionSaveOptions is submitted by the form with the options of the
	// selected theme, each in a field named optionField and the ID of the
	// option, and actionResetOptions by the form that gives the options
	// the defaults of the theme again. Both name the theme they were shown
	// for in the field optionsTheme.
	actionSaveOptions  = "save-options"
	actionResetOptions = "reset-options"
	// actionSaveTemplates is submitted by the form that allows themes to
	// use templates, with a box named templatesAllowed. actionConfirmTemplates
	// and actionWithdrawTemplates are submitted by the form about the
	// templates of the selected theme, which names the theme in the field
	// templatesTheme and the templates it listed in the field templatesFiles.
	actionSaveTemplates     = "save-templates"
	actionConfirmTemplates  = "confirm-templates"
	actionWithdrawTemplates = "withdraw-templates"
	templatesAllowed        = "theme-templates-allowed"
	templatesTheme          = "theme-templates-theme"
	templatesFiles          = "theme-templates-files"
	optionField             = "option-"
	optionsTheme            = "theme-options-theme"
)

// exportPath is where a theme is handed out as a theme file, by its name.
const exportPath = "/export"

func RegisterRouting(e *fiber.App) {
	e.Get(define.SettingPages.Theme.Path, auth.AuthRequired, pageTheme)
	e.Post(define.SettingPages.Theme.Path, auth.AuthRequired, updateThemes)
	e.Get(define.SettingPages.Theme.Path+exportPath, auth.AuthRequired, exportTheme)
}

var updateMu sync.Mutex

func updateThemes(c fiber.Ctx) error {
	updateMu.Lock()
	defer updateMu.Unlock()
	var body struct {
		Theme      string `form:"theme"`
		Action     string `form:"action"`
		Background string `form:"custom-background"`
		Primary    string `form:"custom-primary"`
		Accent     string `form:"custom-accent"`
		Mode       string `form:"theme-mode"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(http.StatusForbidden).JSON("提交数据缺失")
	}
	if body.Action == actionSaveCustom {
		// The form binder borrows PostArgs storage even when Fiber Immutable is enabled.
		// The palette survives the request, so retain owned copies.
		palette := model.Palette{
			Background: strings.Clone(body.Background),
			Primary:    strings.Clone(body.Primary),
			Accent:     strings.Clone(body.Accent),
		}
		return updateCustomPalette(c, palette.Normalized())
	}
	if body.Action == actionImport {
		return importTheme(c, formvalue.Has(c, importReplace))
	}
	if body.Action == actionFetch {
		return fetchTheme(c)
	}
	if body.Action == actionInstall {
		return installTheme(c)
	}
	if body.Action == actionSaveOptions || body.Action == actionResetOptions {
		return updateThemeOptions(c, body.Action == actionResetOptions)
	}
	if body.Action == actionSaveTemplates {
		return updateThemeTemplates(c, formvalue.Has(c, templatesAllowed))
	}
	if body.Action == actionConfirmTemplates || body.Action == actionWithdrawTemplates {
		return confirmThemeTemplates(c, body.Action == actionConfirmTemplates)
	}
	if body.Action == actionSaveMode {
		// The mode is one of three constants once it is accepted, so nothing
		// of the request is retained.
		return updateThemeMode(c, body.Mode)
	}
	// ThemeCurrent survives the request, so retain an owned copy.
	theme := strings.Clone(body.Theme)
	if theme == model.ThemeCustomName {
		return selectCustomPalette(c)
	}
	// The themes directory is read first: a file added or corrected since the
	// page was shown can be selected, and one that went away cannot.
	define.ReloadUserThemes()
	if !define.IsKnownTheme(theme) {
		return c.Status(http.StatusBadRequest).SendString("Unknown theme")
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeName(theme) }); done {
		return err
	}
	define.SetTheme(theme)
	return pageTheme(c)
}

// commitThemeChange runs one settings write with the checks every theme change
// needs. It reports whether the response has already been written; the
// previous theme then remains active.
func commitThemeChange(c fiber.Ctx, save func() error) (bool, error) {
	if auth.PersistentSecurity(c) && !auth.Revalidate(c) {
		return true, c.Status(http.StatusUnauthorized).SendString("AUTH_REQUIRED")
	}
	var saveErr error
	if err := auth.AuthorizedWrite(c, func() error { saveErr = save(); return nil }); err != nil {
		return true, err
	}
	if saveErr != nil {
		failure := settings.ExplainSaveFailure(saveErr)
		return true, c.Status(failure.Status).SendString(failure.Text())
	}
	return false, nil
}

// updateCustomPalette validates, stores and applies the submitted colors. A
// rejected palette is shown again with the reason; nothing is saved.
func updateCustomPalette(c fiber.Ctx, palette model.Palette) error {
	if err := palette.Validate(); err != nil {
		return renderTheme(c, http.StatusBadRequest, themeNotice{submitted: &palette, rejected: err})
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeCustom(palette) }); done {
		return err
	}
	if err := define.SetCustomTheme(palette); err != nil {
		return err
	}
	return renderTheme(c, http.StatusOK, themeNotice{saved: true})
}

// updateThemeMode stores and applies the setting for light and dark.
func updateThemeMode(c fiber.Ctx, submitted string) error {
	mode := ""
	for _, known := range []string{model.ThemeModeAuto, model.ThemeModeLight, model.ThemeModeDark} {
		if submitted == known {
			mode = known
		}
	}
	if mode == "" {
		return c.Status(http.StatusBadRequest).SendString("Unknown theme mode")
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeMode(mode) }); done {
		return err
	}
	define.SetThemeMode(mode)
	return renderTheme(c, http.StatusOK, themeNotice{modeSaved: true})
}

// updateThemeOptions stores the values the administrator chooses for the
// options of the selected theme, or removes them, so that the options have
// the defaults of the theme again. A value is checked against the option as
// the theme has it now; one the option cannot have is refused, and nothing is
// saved. Only values other than the defaults are stored: an option that is
// left alone follows the default of the theme when a later version of the
// theme has another.
func updateThemeOptions(c fiber.Ctx, reset bool) error {
	// The themes directory is read first, so that the options are those of
	// the theme file as it is now.
	define.ReloadUserThemes()
	name, _ := define.ThemeSnapshot()
	// A form is about the theme it was shown for. One that was shown while
	// another theme was selected, in a tab that was left open, is not read
	// as values for the options of the theme that is selected now.
	if formvalue.Get(c, optionsTheme) != name {
		return c.Status(http.StatusBadRequest).SendString("The form is not about the selected theme")
	}
	options, _, known := define.ThemeOptions(name)
	if !known || len(options) == 0 {
		return c.Status(http.StatusBadRequest).SendString("The selected theme has no options")
	}
	chosen := map[string]string{}
	for _, option := range options {
		if reset {
			break
		}
		field := optionField + option.ID
		value := model.ThemeOptionOff
		if option.Kind == model.ThemeOptionBoolean {
			// A box that is not ticked is not submitted.
			if formvalue.Has(c, field) {
				value = model.ThemeOptionOn
			}
		} else if value = formvalue.Get(c, field); option.Value(value) != value {
			// An option has its default for a value it cannot have, so a
			// value that comes back as another one is not one of its values.
			return c.Status(http.StatusBadRequest).SendString("Unknown value for an option of the theme")
		}
		if !option.IsDefault(value) {
			chosen[option.ID] = value
		}
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeOptions(name, chosen) }); done {
		return err
	}
	return renderTheme(c, http.StatusOK, themeNotice{optionsSaved: true})
}

// updateThemeTemplates stores whether themes can use templates. An instance
// that uses none, because it sends no Content-Security-Policy or runs in
// safe mode, does not offer the setting and does not take it.
func updateThemeTemplates(c fiber.Ctx, allowed bool) error {
	if !define.ThemeTemplatesAvailable() {
		return c.Status(http.StatusBadRequest).SendString("This instance uses no templates of themes")
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeTemplatesAllowed(allowed) }); done {
		return err
	}
	return renderTheme(c, http.StatusOK, themeNotice{templatesSaved: true})
}

// confirmThemeTemplates stores that the administrator has confirmed the
// templates of the selected theme, or withdraws that. A confirmation is
// about the templates the page listed: when the theme has others by now,
// nothing is stored and the page lists them again.
func confirmThemeTemplates(c fiber.Ctx, confirmed bool) error {
	// The themes directory is read first, so that the templates are those
	// of the theme as it is now.
	define.ReloadUserThemes()
	name, _ := define.ThemeSnapshot()
	// As with the options: a form that was shown while another theme was
	// selected is not about the theme that is selected now.
	if formvalue.Get(c, templatesTheme) != name {
		return c.Status(http.StatusBadRequest).SendString("The form is not about the selected theme")
	}
	if confirmed {
		files := define.ThemeTemplateFiles(name)
		options, err := data.GetAllSettingsOptions()
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString("config error")
		}
		if !define.ThemeTemplatesAvailable() || !options.ThemeTemplates || len(files) == 0 {
			return c.Status(http.StatusBadRequest).SendString("The selected theme has no templates that can be used")
		}
		if formvalue.Get(c, templatesFiles) != strings.Join(files, ",") {
			return renderTheme(c, http.StatusConflict, themeNotice{templatesNotice: "theme_templates_changed"})
		}
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeTemplatesConfirmed(name, confirmed) }); done {
		return err
	}
	notice := "theme_templates_withdraw_done"
	if confirmed {
		notice = "theme_templates_confirm_done"
	}
	return renderTheme(c, http.StatusOK, themeNotice{templatesNotice: notice})
}

// importTheme stores an uploaded theme file in the themes directory. The
// name of the theme is the name of the uploaded file. The file is checked
// like the files of the directory, and it is not selected by being stored.
// A refusal is shown on the page with its reason, and nothing is written.
func importTheme(c fiber.Ctx, replace bool) error {
	locale := settingsLocale()
	refuse := func(status int, text string) error {
		return renderTheme(c, status, themeNotice{importRefused: text})
	}
	header, err := c.FormFile(importField)
	if err != nil || header.Filename == "" {
		return refuse(http.StatusBadRequest, i18n.T(locale, "theme_import_missing"))
	}
	// A browser may send the path the file has on the device it comes from.
	// Only what follows the last separator of either kind is the name.
	filename := header.Filename
	if separator := strings.LastIndexAny(filename, `/\`); separator >= 0 {
		filename = filename[separator+1:]
	}
	name, isThemeFile := strings.CutSuffix(filename, ".yml")
	if !isThemeFile {
		// A theme that is a directory comes as an archive.
		if archived, isArchive := strings.CutSuffix(filename, ".zip"); isArchive {
			return importThemeArchive(c, header, archived, replace)
		}
		return refuse(http.StatusBadRequest, i18n.T(locale, "theme_import_name"))
	}
	if header.Size > define.MaximumThemeFileBytes {
		return refuse(http.StatusBadRequest, i18n.T(locale, "theme_import_size"))
	}
	upload, err := header.Open()
	if err != nil {
		return refuse(http.StatusBadRequest, i18n.T(locale, "theme_import_missing"))
	}
	raw, err := io.ReadAll(io.LimitReader(upload, define.MaximumThemeFileBytes+1))
	_ = upload.Close()
	if err != nil {
		return refuse(http.StatusBadRequest, i18n.T(locale, "theme_import_missing"))
	}
	// The name is kept beyond the request, in the list of themes.
	name = strings.Clone(name)
	var stored error
	var withdrawn []string
	if done, err := commitThemeChange(c, func() error {
		if stored = define.StoreUserTheme(name, raw, replace); stored != nil {
			return nil
		}
		var err error
		withdrawn, err = withdrawImportedTemplates(name)
		return err
	}); done {
		return err
	}
	if stored != nil {
		return refuse(importRefusal(locale, stored))
	}
	return renderTheme(c, http.StatusOK, themeNotice{imported: name, withdrawn: withdrawn})
}

// importThemeArchive stores the theme of an uploaded archive in the themes
// directory, as a directory of the name of the uploaded file. Only the files
// of a theme are stored, and they are checked like a theme of the directory.
// The entries that were left out are named on the page.
func importThemeArchive(c fiber.Ctx, header *multipart.FileHeader, name string, replace bool) error {
	locale := settingsLocale()
	refuse := func(status int, text string) error {
		return renderTheme(c, status, themeNotice{importRefused: text})
	}
	if header.Size > model.MaximumThemeArchiveBytes {
		return refuse(http.StatusBadRequest, i18n.T(locale, "theme_import_archive_size"))
	}
	upload, err := header.Open()
	if err != nil {
		return refuse(http.StatusBadRequest, i18n.T(locale, "theme_import_missing"))
	}
	defer upload.Close()
	// The name is kept beyond the request, in the list of themes.
	name = strings.Clone(name)
	var stored error
	var leftOut []string
	var withdrawn []string
	if done, err := commitThemeChange(c, func() error {
		if leftOut, stored = define.StoreUserThemeArchive(name, upload, header.Size, replace); stored != nil {
			return nil
		}
		var err error
		withdrawn, err = withdrawImportedTemplates(name)
		return err
	}); done {
		return err
	}
	if stored != nil {
		return refuse(importRefusal(locale, stored))
	}
	return renderTheme(c, http.StatusOK, themeNotice{imported: name, leftOut: leftOut, withdrawn: withdrawn})
}

// withdrawImportedTemplates withdraws the confirmation of the templates of
// a theme that was imported, and of the themes that have it below: what was
// confirmed are the templates that were there, and an import brings others.
// It returns the themes a confirmation was withdrawn for.
func withdrawImportedTemplates(name string) ([]string, error) {
	return data.WithdrawThemeTemplates(define.ThemesShownWith(name))
}

// importRefusal words why a theme was not imported, with the status of the
// response. A file that is not a usable part of a theme is named with the
// reason the list of files that are not used gives for such a file, after
// the statement every refusal makes.
func importRefusal(locale string, stored error) (int, string) {
	var rejection *model.ThemeFileError
	var entry *define.ThemeArchiveFileError
	var part *define.ThemeArchiveProblem
	rejected := func(problem define.ThemeProblem) string {
		text := i18n.Format(locale, "theme_import_rejected", map[string]any{"reason": i18n.T(locale, "theme_file_problem_"+problem.Reason)})
		for _, detail := range []string{problem.File, problem.Subject} {
			if detail != "" {
				text += " " + plainText(detail, maximumShownDetail)
			}
		}
		return text
	}
	switch {
	case errors.Is(stored, define.ErrThemeStoreName):
		return http.StatusBadRequest, i18n.T(locale, "theme_import_name")
	case errors.Is(stored, define.ErrThemeStoreSize):
		return http.StatusBadRequest, i18n.T(locale, "theme_import_size")
	case errors.Is(stored, define.ErrThemeStoreExists):
		return http.StatusConflict, i18n.T(locale, "theme_import_exists")
	case errors.Is(stored, define.ErrThemeStoreDirectory):
		return http.StatusConflict, i18n.T(locale, "theme_import_directory")
	case errors.Is(stored, define.ErrThemeStoreOccupied):
		return http.StatusConflict, i18n.T(locale, "theme_import_occupied")
	case errors.Is(stored, define.ErrThemeStoreSafeMode):
		return http.StatusConflict, i18n.T(locale, "theme_import_safe_mode")
	case errors.Is(stored, define.ErrThemeStoreFull):
		return http.StatusConflict, i18n.T(locale, "theme_import_full")
	case errors.Is(stored, define.ErrThemeArchiveSize):
		return http.StatusBadRequest, i18n.T(locale, "theme_import_archive_size")
	case errors.Is(stored, define.ErrThemeArchiveEntries):
		return http.StatusBadRequest, i18n.T(locale, "theme_import_archive_entries")
	case errors.Is(stored, define.ErrThemeArchiveEmpty):
		return http.StatusBadRequest, i18n.T(locale, "theme_import_archive_empty")
	case errors.Is(stored, define.ErrThemeArchiveSeveral):
		return http.StatusBadRequest, i18n.T(locale, "theme_import_archive_several")
	case errors.As(stored, &entry):
		// A refusal about one file of the archive names the file.
		key := "theme_import_archive_read"
		if errors.Is(stored, define.ErrThemeArchiveTwice) {
			key = "theme_import_archive_twice"
		} else if errors.Is(stored, define.ErrThemeArchiveDocument) {
			key = "theme_import_archive_document"
		}
		return http.StatusBadRequest, i18n.T(locale, key) + " " + plainText(entry.File, maximumShownDetail)
	case errors.Is(stored, define.ErrThemeArchiveRead):
		return http.StatusBadRequest, i18n.T(locale, "theme_import_archive_read")
	case errors.As(stored, &part):
		return http.StatusBadRequest, rejected(part.Problem)
	case errors.As(stored, &rejection):
		return http.StatusBadRequest, rejected(define.ThemeFileProblem(stored))
	default:
		return http.StatusInternalServerError, i18n.T(locale, "theme_import_write")
	}
}

// exportTheme hands a theme out to download: a theme file of the themes
// directory as it is, the file a bundled theme is defined by, the custom
// colors written as a file, or a theme that is a directory as an archive.
func exportTheme(c fiber.Ctx) error {
	if auth.PersistentSecurity(c) && !auth.Revalidate(c) {
		return c.Status(http.StatusUnauthorized).SendString("AUTH_REQUIRED")
	}
	// The directory is read first, so that the file is the one there now.
	define.ReloadUserThemes()
	contentType := "application/yaml; charset=utf-8"
	filename, raw, known := define.ExportTheme(c.Query("name"))
	if !known {
		// A theme that is a directory is handed out as an archive.
		archive, packed, err := define.ExportThemeArchive(c.Query("name"))
		switch {
		case errors.Is(err, define.ErrThemeArchiveSize):
			return c.Status(http.StatusConflict).SendString(i18n.T(settingsLocale(), "theme_export_archive_size"))
		case errors.Is(err, define.ErrThemeArchiveRead):
			return c.Status(http.StatusConflict).SendString(i18n.T(settingsLocale(), "theme_export_archive_read"))
		case err != nil:
			return c.Status(http.StatusNotFound).SendString("Unknown theme")
		}
		filename, raw, contentType = archive, packed, "application/zip"
	}
	// The file name consists of a theme name and fixed text.
	c.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Set("Cache-Control", "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Type", contentType)
	return c.Status(http.StatusOK).Send(raw)
}

// settingsLocale returns the interface language of the settings pages.
func settingsLocale() string {
	options, err := data.GetAllSettingsOptions()
	if err != nil || options.Locale == "" {
		return "zh"
	}
	return options.Locale
}

// selectCustomPalette applies the palette stored in config.yml. The file is
// the source of truth: it may have been edited since the palette was last
// applied, and the selection has to match what the next start will read.
func selectCustomPalette(c fiber.Ctx) error {
	options, readErr := data.GetAllSettingsOptions()
	if readErr != nil {
		failure := settings.ExplainSaveFailure(settings.ConfigUnreadable(readErr))
		return c.Status(failure.Status).SendString(failure.Text())
	}
	palette, stored := storedCustomPalette(options)
	if !stored {
		return c.Status(http.StatusBadRequest).SendString("Unknown theme")
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeName(model.ThemeCustomName) }); done {
		return err
	}
	if err := define.SetCustomTheme(palette); err != nil {
		return err
	}
	return pageTheme(c)
}

// storedCustomPalette returns the custom palette in the configuration if it
// can be used. A hand-edited block that fails validation is treated as absent.
func storedCustomPalette(options model.Application) (model.Palette, bool) {
	if options.ThemeCustom == nil {
		return model.Palette{}, false
	}
	palette := options.ThemeCustom.Normalized()
	if palette.Validate() != nil {
		return model.Palette{}, false
	}
	return palette, true
}

// themeNotice carries the outcome of a custom color submission to the page.
type themeNotice struct {
	// submitted keeps the colors of a rejected palette in the form.
	submitted *model.Palette
	rejected  error
	saved     bool
	// modeSaved reports a stored setting for light and dark.
	modeSaved bool
	// imported names a theme file that was stored, and importRefused says
	// why one was not.
	imported      string
	importRefused string
	// leftOut names the entries of an imported archive that are no files of
	// a theme and were not stored.
	leftOut []string
	// withdrawn names the themes whose templates are no longer confirmed
	// after an import.
	withdrawn []string
	// installAddress and installReplace are what the form that fetches a
	// theme was sent with, installRefused says why nothing was fetched or
	// installed, and installOffer is a theme that was fetched.
	installAddress string
	installReplace bool
	installRefused string
	installOffer   *themeInstallOffer
	// optionsSaved reports stored values for the options of the theme.
	optionsSaved bool
	// templatesSaved reports a stored setting for the templates of themes,
	// and templatesNotice is the key of what the form about the templates
	// of the selected theme says.
	templatesSaved  bool
	templatesNotice string
}

// themeTemplatesSwitch is the form that allows themes to use templates.
type themeTemplatesSwitch struct {
	Action  string
	Field   string
	Allowed bool
	Saved   bool
	// Unavailable says why this instance uses no templates. The setting is
	// not offered then.
	Unavailable string
}

// themeTemplatesConfirm is the form about the templates of the selected
// theme: it lists them, says what they can do, and confirms them or
// withdraws the confirmation.
type themeTemplatesConfirm struct {
	Action     string
	ThemeField string
	Theme      string
	FilesField string
	// Files are the templates the form is about, as it submits them, and
	// Listed is the sentence that names them.
	Files     string
	Listed    string
	Title     string
	Notice    string
	Submit    string
	Confirmed bool
	// Preview is the address of the home page, which a form opens with the
	// theme named in the field PreviewField to show the templates before
	// they are confirmed.
	Preview      string
	PreviewField string
}

func pageTheme(c fiber.Ctx) error {
	return renderTheme(c, http.StatusOK, themeNotice{})
}

func renderTheme(c fiber.Ctx, status int, notice themeNotice) error {
	themes := define.ThemePalettes
	// The page lists the theme files as they are now.
	define.ReloadUserThemes()
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	m := pool.GetTemplateMap()
	if auth.PersistentSecurity(c) {
		m["SecurityCSRFToken"] = auth.CSRFToken(c)
	}
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	pageTheme := define.GetPageTheme()
	m["PageAppearance"] = pageTheme.BodyStyle
	m["PageThemeRules"] = pageTheme.Rules
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["PageName"] = "Theme"
	m["SettingPages"] = define.SettingPages
	m["Themes"] = themes
	m["OptionTitle"] = options.Title
	fileThemes, problems, notices := define.UserThemes()
	m["ThemeFiles"] = themeFileCards(locale, fileThemes, options)
	if selected, _ := define.ThemeSnapshot(); selected != "" {
		// A template that passed its check and did not compile for the
		// settings of a page is listed with the other remarks.
		problems = append(problems, define.ThemePageTemplateProblems(selected)...)
	}
	m["ThemeFileLines"] = themeFileLines(locale, problems, notices)
	templates := themeTemplatesSwitch{Action: actionSaveTemplates, Field: templatesAllowed, Allowed: options.ThemeTemplates, Saved: notice.templatesSaved}
	switch {
	case define.AppFlags.ThemeSafeMode:
		templates.Unavailable = i18n.T(locale, "theme_templates_unavailable_safe_mode")
	case !define.ThemeTemplatesAvailable():
		templates.Unavailable = i18n.T(locale, "theme_templates_unavailable_csp")
	}
	m["ThemeTemplates"] = templates
	if selected, _ := define.ThemeSnapshot(); templates.Unavailable == "" && options.ThemeTemplates {
		if files := define.ThemeTemplateFiles(selected); len(files) > 0 {
			confirm := themeTemplatesConfirm{
				Action: actionConfirmTemplates, ThemeField: templatesTheme, Theme: selected,
				FilesField: templatesFiles, Files: strings.Join(files, ","),
				Listed:    i18n.Format(locale, "theme_templates_confirm_files", map[string]any{"files": strings.Join(files, ", ")}),
				Title:     i18n.Format(locale, "theme_templates_confirm_title", map[string]any{"name": define.ThemeTitle(selected, locale)}),
				Submit:    i18n.T(locale, "theme_templates_confirm_submit"),
				Confirmed: options.ThemeSettings[selected].Templates,
			}
			if !confirm.Confirmed {
				confirm.Preview, confirm.PreviewField = define.RegularPages.Home.Path, "theme-preview"
			}
			if confirm.Confirmed {
				confirm.Action, confirm.Submit = actionWithdrawTemplates, i18n.T(locale, "theme_templates_withdraw_submit")
			}
			if notice.templatesNotice != "" {
				confirm.Notice = i18n.T(locale, notice.templatesNotice)
			}
			m["ThemeTemplatesConfirm"] = confirm
		}
	}
	// The setting for light and dark is offered while it has an effect: on a
	// theme with different values for the two.
	mode, choice := define.ThemeModeState()
	m["ThemeMode"] = mode
	m["ThemeModeChoice"] = choice
	m["ThemeModeAction"] = actionSaveMode
	m["ThemeModeSaved"] = notice.modeSaved
	m["ThemeExportPath"] = define.SettingPages.Theme.Path + exportPath
	m["ThemeImportAction"] = actionImport
	m["ThemeInstall"] = installForm(locale, notice)
	m["ThemeImportRefused"] = notice.importRefused
	if notice.imported != "" {
		m["ThemeImported"] = i18n.Format(locale, "theme_import_done", map[string]any{"name": define.ThemeTitle(notice.imported, locale)})
	}
	if len(notice.withdrawn) > 0 {
		titles := make([]string, 0, len(notice.withdrawn))
		for _, theme := range notice.withdrawn {
			titles = append(titles, define.ThemeTitle(theme, locale))
		}
		m["ThemeImportWithdrawn"] = i18n.Format(locale, "theme_import_withdrawn", map[string]any{"themes": strings.Join(titles, i18n.T(locale, "list_separator"))})
	}
	if len(notice.leftOut) > 0 {
		m["ThemeImportLeftOut"] = i18n.Format(locale, "theme_import_left_out", map[string]any{"files": leftOutText(notice.leftOut)})
	}
	if selected, _ := define.ThemeSnapshot(); selected != "" && !define.IsKnownTheme(selected) {
		reason := "theme_unavailable"
		// In safe mode the file of the theme is not what is wrong.
		if define.AppFlags.ThemeSafeMode && define.IsThemeOfDirectory(selected) {
			reason = "theme_safe_mode_selected"
		}
		m["ThemeUnavailable"] = i18n.Format(locale, reason, map[string]any{"name": plainText(selected, maximumShownName)})
	}
	if define.AppFlags.ThemeSafeMode {
		m["ThemeSafeMode"] = i18n.T(locale, "theme_safe_mode_notice")
	}
	// The options of the selected theme are offered with the values they
	// have now.
	if selected, _ := define.ThemeSnapshot(); selected != "" {
		if declared, values, known := define.ThemeOptions(selected); known && len(declared) > 0 {
			m["ThemeOptions"] = themeOptionFields(locale, declared, values)
			m["ThemeOptionsTitle"] = i18n.Format(locale, "theme_options_title", map[string]any{"name": define.ThemeTitle(selected, locale)})
			m["ThemeOptionsTheme"] = selected
			m["ThemeOptionsAction"] = actionSaveOptions
			m["ThemeOptionsReset"] = actionResetOptions
			m["ThemeOptionsSaved"] = notice.optionsSaved
		}
	}

	if custom, stored := storedCustomPalette(options); stored {
		m["ThemeCustom"] = custom
	}
	// The form starts from the colors in use, so that any theme, bundled or
	// custom, can be adjusted.
	form := define.CurrentThemePalette().Normalized()
	if notice.submitted != nil {
		form = keepColors(form, *notice.submitted)
	}
	m["ThemeCustomForm"] = form
	m["ThemeCustomAction"] = actionSaveCustom
	m["ThemeCustomSaved"] = notice.saved
	if notice.rejected != nil {
		m["ThemeCustomError"] = paletteErrorText(locale, notice.rejected)
	}
	return c.Status(status).Render("settings.html", m)
}

// themeOptionField is an option of the selected theme as the page offers it:
// a box to tick for a boolean option, a list for an option with values, and a
// color to pick for a color option.
type themeOptionField struct {
	// ID is the ID of the option, which names the field of the form.
	ID    string
	Kind  string
	Label string
	// On is the state of a boolean option, Value the color of a color
	// option, and Choices are the values of an option with values.
	On      bool
	Value   string
	Choices []themeOptionFieldChoice
}

// themeOptionFieldChoice is a value of an option, and whether it is the one
// the option has now.
type themeOptionFieldChoice struct {
	Value    string
	Label    string
	Selected bool
}

// themeOptionFields prepares the options of a theme for the page, with the
// values they have now and the labels of the interface language.
func themeOptionFields(locale string, options []model.ThemeOption, values map[string]string) []themeOptionField {
	fields := make([]themeOptionField, 0, len(options))
	for _, option := range options {
		value := values[option.ID]
		field := themeOptionField{ID: option.ID, Kind: option.Kind, Label: option.DisplayName(locale)}
		switch option.Kind {
		case model.ThemeOptionBoolean:
			field.On = value == model.ThemeOptionOn
		case model.ThemeOptionColor:
			field.Value = value
		default:
			for _, choice := range option.Choices {
				field.Choices = append(field.Choices, themeOptionFieldChoice{Value: choice.Value, Label: choice.DisplayName(locale), Selected: choice.Value == value})
			}
		}
		fields = append(fields, field)
	}
	return fields
}

// keepColors overlays the submitted values that are colors. A color input
// cannot show anything else, so such a field keeps its previous color rather
// than echoing the rejected text back into the page.
func keepColors(form, submitted model.Palette) model.Palette {
	keep := func(current, value string) string {
		if model.IsHexColor(value) {
			return value
		}
		return current
	}
	return model.Palette{
		Background: keep(form.Background, submitted.Background),
		Primary:    keep(form.Primary, submitted.Primary),
		Accent:     keep(form.Accent, submitted.Accent),
	}
}

func paletteErrorText(locale string, err error) string {
	if errors.Is(err, model.ErrPaletteContrast) {
		return i18n.T(locale, "theme_custom_low_contrast")
	}
	return i18n.T(locale, "theme_custom_invalid_color")
}

// maximumLeftOutShown is the number of entries of an archive the page names
// as left out. An archive can hold many more.
const maximumLeftOutShown = 12

// leftOutText names the entries of an archive that were left out, as one
// line of text of a bounded length: the names come from the archive.
func leftOutText(entries []string) string {
	shown := make([]string, 0, maximumLeftOutShown+1)
	for i, entry := range entries {
		if i == maximumLeftOutShown {
			shown = append(shown, "…")
			break
		}
		shown = append(shown, plainText(entry, maximumShownName))
	}
	return strings.Join(shown, ", ")
}

// themeFileCard is a theme of the themes directory in use, as the page shows
// it.
type themeFileCard struct {
	// Name is the name of the file without its extension, or the name of
	// the directory: the value stored in config.yml when the theme is
	// selected.
	Name string
	// Title is the name to show, About the version, author and license the
	// file gives.
	Title string
	About string
	// Colors are the colors of the theme in light mode. Dark holds those of
	// dark mode when they are different.
	Colors model.Palette
	Dark   *model.Palette
	// Layout names the layout options the theme specifies, so that it is
	// known before the theme is selected that it changes more than colors.
	Layout string
	// Unsupported names the layout options the templates of the theme do
	// not support.
	Unsupported string
	// Stylesheet reports a theme that brings a stylesheet, so that it is
	// known before the theme is selected that it changes more than colors.
	Stylesheet bool
	// Options reports a theme with options, which are offered on this page
	// once the theme is selected.
	Options bool
	// Based says which theme the theme is based on, and is empty for a
	// theme that stands on its own.
	Based string
	// Templates names the page templates the theme brings and says whether
	// they are used.
	Templates string
}

func themeFileCards(locale string, themes []model.Theme, options model.Application) []themeFileCard {
	cards := make([]themeFileCard, 0, len(themes))
	// A theme of the directory is named as its card names it, and a bundled
	// theme by its name.
	titles := make(map[string]string, len(themes))
	for _, theme := range themes {
		titles[theme.Name] = theme.DisplayName(locale)
	}
	for _, theme := range themes {
		about := make([]string, 0, 3)
		for _, text := range []string{theme.Version, theme.Author, theme.License} {
			if text != "" {
				about = append(about, text)
			}
		}
		card := themeFileCard{
			Name:       theme.Name,
			Title:      theme.DisplayName(locale),
			About:      strings.Join(about, " · "),
			Colors:     theme.In(model.ThemeModeLight).Colors,
			Stylesheet: theme.Stylesheet != "",
			Options:    len(theme.Options) > 0,
		}
		if dark := theme.In(model.ThemeModeDark).Colors; dark != card.Colors {
			card.Dark = &dark
		}
		if theme.Extends != "" {
			base := titles[theme.Extends]
			if base == "" {
				base = theme.Extends
			}
			card.Based = i18n.Format(locale, "theme_extends_note", map[string]any{"name": base})
		}
		if len(theme.Templates) > 0 {
			files := make([]string, 0, len(theme.Templates))
			for _, template := range theme.Templates {
				files = append(files, template.Label(theme.Name))
			}
			note := "theme_templates_note"
			switch {
			case !define.ThemeTemplatesAvailable():
				note = "theme_templates_note_unavailable"
			case options.ThemeTemplates && options.ThemeSettings[theme.Name].Templates:
				note = "theme_templates_note_used"
			case options.ThemeTemplates:
				note = "theme_templates_note_unconfirmed"
			}
			card.Templates = i18n.Format(locale, note, map[string]any{"files": strings.Join(files, ", ")})
			if unsupported := model.ThemeLayoutUnsupported(theme.Supports, theme.Layout); len(unsupported) > 0 {
				card.Unsupported = i18n.Format(locale, "theme_layout_unsupported_note", map[string]any{"options": settings.LayoutOptionList(locale, unsupported)})
			}
		}
		if specified := (*model.HomepageOptions)(nil).ThemeLayoutApplied(theme.Layout, nil); len(specified) > 0 {
			card.Layout = i18n.Format(locale, "theme_layout_sets", map[string]any{"options": settings.LayoutOptionList(locale, specified)})
		}
		cards = append(cards, card)
	}
	return cards
}

// themeFileLine is one remark about a file in the themes directory: why it
// is not in use, or which of its parts are not used.
type themeFileLine struct {
	File   string
	Text   string
	Detail string
}

// Longest file name and longest detail shown for a file. Both come from the
// themes directory; the page shows them as plain text of a bounded length.
// The longest path of a file that can be in use fits: an image or a font in
// the assets of a theme.
const (
	maximumShownName   = 104
	maximumShownDetail = 240
)

func themeFileLines(locale string, problems []define.ThemeProblem, notices []define.ThemeNotice) []themeFileLine {
	lines := make([]themeFileLine, 0, len(problems)+len(notices))
	for _, problem := range problems {
		lines = append(lines, themeFileLine{
			File:   plainText(problem.File, maximumShownName),
			Text:   i18n.T(locale, "theme_file_problem_"+problem.Reason),
			Detail: plainText(problem.Subject, maximumShownDetail),
		})
	}
	for _, notice := range notices {
		parts := append(append([]string(nil), notice.Unused.Keys...), notice.Unused.Tokens...)
		lines = append(lines, themeFileLine{
			File:   plainText(notice.File, maximumShownName),
			Text:   i18n.T(locale, "theme_file_unused"),
			Detail: plainText(strings.Join(parts, ", "), maximumShownDetail),
		})
	}
	// One file can have a problem and unused parts; its lines stay together.
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].File < lines[j].File })
	return lines
}

// plainText prepares a file name or a parser message for display: one line
// of valid text, cut to a number of characters.
func plainText(text string, limit int) string {
	text = strings.Join(strings.FieldsFunc(strings.ToValidUTF8(text, "?"), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit]) + "…"
}
