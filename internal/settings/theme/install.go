package theme

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/formvalue"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/themefetch"
)

// actionFetch is submitted by the form that fetches a theme from the
// address in the field installAddress, and actionInstall by the form the
// page shows for what was fetched, which names the address again and the
// SHA-256 of what was shown in the field installHash. Both carry the box
// importReplace.
const (
	actionFetch    = "fetch-theme"
	actionInstall  = "install-theme"
	installAddress = "theme-address"
	installHash    = "theme-hash"
	// maximumInstallAddress is the longest address that is read.
	maximumInstallAddress = 2048
	// installTimeout bounds one download with everything around it.
	installTimeout = 40 * time.Second
)

// themeInstallOffer is a theme that was fetched and is not installed.
type themeInstallOffer struct {
	address  string
	download themefetch.Download
	contents define.ThemeArchiveContents
	replace  bool
}

// installUnavailable says why this instance fetches no theme, or nothing
// where it does: an instance in offline mode makes no request, and one in
// safe mode stores no theme.
func installUnavailable(locale string) string {
	switch {
	case define.AppFlags.EnableOfflineMode:
		return i18n.T(locale, "theme_install_offline")
	case define.AppFlags.ThemeSafeMode:
		return i18n.T(locale, "theme_import_safe_mode")
	}
	return ""
}

// downloadTheme fetches the theme at the address of the form, for the
// administrator, and checks what can be checked without storing it: the
// name, and that the content is a theme file or the archive of one theme.
// It reports whether the response has been written; otherwise it returns
// the download or the status and the text of a refusal.
func downloadTheme(c fiber.Ctx, locale string) (themeInstallOffer, int, string, bool, error) {
	offer := themeInstallOffer{address: strings.Clone(formvalue.Get(c, installAddress)), replace: formvalue.Has(c, importReplace)}
	if text := installUnavailable(locale); text != "" {
		return offer, http.StatusConflict, text, false, nil
	}
	if len(offer.address) > maximumInstallAddress {
		return offer, http.StatusBadRequest, i18n.T(locale, "theme_install_refused_address"), false, nil
	}
	_, name, _, err := themefetch.ParseAddress(offer.address)
	if err == nil && !define.IsUserThemeName(name) {
		err = themefetch.ErrName
	}
	if err != nil {
		status, text := fetchRefusal(locale, err)
		return offer, status, text, false, nil
	}
	// The request goes out for the administrator only, like a write.
	var fetched error
	if done, written := commitThemeChange(c, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
		defer cancel()
		offer.download, fetched = themefetch.Default.Fetch(ctx, offer.address, define.MaximumThemeFileBytes, model.MaximumThemeArchiveBytes)
		return nil
	}); done {
		return offer, 0, "", true, written
	}
	if fetched != nil {
		status, text := fetchRefusal(locale, fetched)
		return offer, status, text, false, nil
	}
	if offer.download.Archive {
		offer.contents, err = define.InspectThemeArchive(bytes.NewReader(offer.download.Raw), int64(len(offer.download.Raw)))
	} else {
		err = define.CheckUserThemeFile(offer.download.Raw)
	}
	if err != nil {
		status, text := importRefusal(locale, err)
		return offer, status, text, false, nil
	}
	return offer, http.StatusOK, "", false, nil
}

// fetchTheme fetches a theme from an address and shows what was fetched,
// with its SHA-256. Nothing is stored.
func fetchTheme(c fiber.Ctx) error {
	offer, status, refused, done, err := downloadTheme(c, settingsLocale())
	if done {
		return err
	}
	if refused != "" {
		return renderTheme(c, status, themeNotice{installAddress: offer.address, installReplace: offer.replace, installRefused: refused})
	}
	return renderTheme(c, http.StatusOK, themeNotice{installAddress: offer.address, installReplace: offer.replace, installOffer: &offer})
}

// installTheme fetches the theme again and stores it if it is what the
// page showed: the SHA-256 of the form has to be that of the content. It
// is stored as an upload of the same content would be.
func installTheme(c fiber.Ctx) error {
	locale := settingsLocale()
	offer, status, refused, done, err := downloadTheme(c, locale)
	if done {
		return err
	}
	if refused == "" && offer.download.SHA256 != formvalue.Get(c, installHash) {
		status, refused = http.StatusConflict, i18n.T(locale, "theme_install_changed")
	}
	if refused != "" {
		return renderTheme(c, status, themeNotice{installAddress: offer.address, installReplace: offer.replace, installRefused: refused})
	}
	name := strings.Clone(offer.download.Name)
	var stored error
	var leftOut, withdrawn []string
	if done, err := commitThemeChange(c, func() error {
		if offer.download.Archive {
			leftOut, stored = define.StoreUserThemeArchive(name, bytes.NewReader(offer.download.Raw), int64(len(offer.download.Raw)), offer.replace)
		} else {
			stored = define.StoreUserTheme(name, offer.download.Raw, offer.replace)
		}
		if stored != nil {
			return nil
		}
		var err error
		withdrawn, err = withdrawImportedTemplates(name)
		return err
	}); done {
		return err
	}
	if stored != nil {
		status, refused = importRefusal(locale, stored)
		return renderTheme(c, status, themeNotice{installAddress: offer.address, installReplace: offer.replace, installRefused: refused})
	}
	return renderTheme(c, http.StatusOK, themeNotice{imported: name, leftOut: leftOut, withdrawn: withdrawn})
}

// fetchRefusal words why nothing was fetched, with the status of the
// response.
func fetchRefusal(locale string, reason error) (int, string) {
	switch {
	case errors.Is(reason, themefetch.ErrAddress):
		return http.StatusBadRequest, i18n.T(locale, "theme_install_refused_address")
	case errors.Is(reason, themefetch.ErrName):
		return http.StatusBadRequest, i18n.T(locale, "theme_install_refused_name")
	case errors.Is(reason, themefetch.ErrDestination):
		return http.StatusBadRequest, i18n.T(locale, "theme_install_refused_destination")
	case errors.Is(reason, themefetch.ErrRedirect):
		return http.StatusBadGateway, i18n.T(locale, "theme_install_refused_redirect")
	case errors.Is(reason, themefetch.ErrStatus):
		return http.StatusBadGateway, i18n.T(locale, "theme_install_refused_status")
	case errors.Is(reason, themefetch.ErrSize):
		return http.StatusBadRequest, i18n.T(locale, "theme_install_refused_size")
	default:
		return http.StatusBadGateway, i18n.T(locale, "theme_install_refused_failed")
	}
}

// themeInstallForm is the form that fetches a theme from an address, with
// what the page shows of a theme that was fetched.
type themeInstallForm struct {
	Action, AddressField, Address string
	ReplaceField                  string
	Replace                       bool
	Refused, Unavailable          string
	Offer                         *themeInstallOfferView
}

// themeInstallOfferView is a theme that was fetched, as the page shows it
// above the button that installs it.
type themeInstallOfferView struct {
	Action, HashField, Hash string
	Lines                   []string
	LeftOut, Templates      string
}

func installForm(locale string, notice themeNotice) themeInstallForm {
	form := themeInstallForm{
		Action: actionFetch, AddressField: installAddress, Address: notice.installAddress, ReplaceField: importReplace, Replace: notice.installReplace,
		Refused: notice.installRefused, Unavailable: installUnavailable(locale),
	}
	offer := notice.installOffer
	if offer == nil {
		return form
	}
	view := &themeInstallOfferView{Action: actionInstall, HashField: installHash, Hash: offer.download.SHA256}
	view.Lines = []string{
		i18n.Format(locale, "theme_install_offer_name", map[string]any{"name": offer.download.Name}),
		i18n.Format(locale, "theme_install_offer_size", map[string]any{"size": strconv.Itoa(len(offer.download.Raw))}),
		i18n.Format(locale, "theme_install_offer_hash", map[string]any{"hash": offer.download.SHA256}),
	}
	if offer.download.Archive {
		stylesheet := i18n.T(locale, "theme_install_no")
		if offer.contents.Stylesheet {
			stylesheet = i18n.T(locale, "theme_install_yes")
		}
		templates := i18n.T(locale, "theme_install_none")
		if len(offer.contents.Templates) > 0 {
			templates = strings.Join(offer.contents.Templates, ", ")
			view.Templates = i18n.T(locale, "theme_install_templates_note")
		}
		view.Lines = append(view.Lines, i18n.Format(locale, "theme_install_offer_archive", map[string]any{
			"stylesheet": stylesheet, "assets": strconv.Itoa(offer.contents.Assets), "documents": strconv.Itoa(offer.contents.Documents), "templates": templates,
		}))
		if len(offer.contents.Others) > 0 {
			view.LeftOut = i18n.Format(locale, "theme_install_left_out", map[string]any{"files": leftOutText(offer.contents.Others)})
		}
	} else {
		view.Lines = append(view.Lines, i18n.T(locale, "theme_install_offer_file"))
	}
	form.Offer = view
	return form
}
