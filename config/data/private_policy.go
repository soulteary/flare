package data

import (
	"errors"
	"sync/atomic"

	"github.com/soulteary/flare/config/model"
)

// ErrPrivateContentRequiresLogin reports a collection that contains private
// categories or entries while no visitor can be authenticated.
var ErrPrivateContentRequiresLogin = errors.New("private navigation content requires login")

var privateContentRequiresLogin atomic.Bool

// RequireLoginForPrivateContent is set at startup. With login disabled every
// visitor has the same access, so a private entry cannot be shown to one of
// them without being shown to all. Readers, the editor and import/export then
// refuse the whole collection rather than filtering it: a filtered editor
// document would silently drop the private entries on its next save.
func RequireLoginForPrivateContent(required bool) { privateContentRequiresLogin.Store(required) }

func containsPrivateNavigation(value model.Bookmarks) bool {
	for _, c := range value.Categories {
		if c.Private {
			return true
		}
	}
	for _, b := range value.Items {
		if b.Private {
			return true
		}
	}
	return false
}

// privateContentError applies the policy to a decoded collection.
func privateContentError(value model.Bookmarks) error {
	if privateContentRequiresLogin.Load() && containsPrivateNavigation(value) {
		return ErrPrivateContentRequiresLogin
	}
	return nil
}

// withheld reports whether the policy forbids every reader of this snapshot.
// The scan runs once per immutable snapshot.
func (snapshot *parsedBookmarks) withheld() bool {
	if !privateContentRequiresLogin.Load() {
		return false
	}
	snapshot.privateOnce.Do(func() { snapshot.private = containsPrivateNavigation(snapshot.value) })
	return snapshot.private
}
