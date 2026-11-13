package data

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func writeNavigationFile(tb testing.TB, name, content string) {
	tb.Helper()
	require.NoError(tb, os.WriteFile(name+".yml", []byte(content), 0600))
}

// settleNavigationFile gives a fixture a modification time old enough for a
// matching stamp to be trusted, as on a server that has been running a while.
func settleNavigationFile(tb testing.TB, name string, age time.Duration) {
	tb.Helper()
	settled := time.Now().Add(-age).Truncate(time.Second)
	require.NoError(tb, os.Chtimes(name+".yml", settled, settled))
}

// unsettleNavigationFile keeps a fixture inside the window in which a matching
// stamp is not trusted. A future time holds however slowly the test runs.
func unsettleNavigationFile(tb testing.TB, name string) time.Time {
	tb.Helper()
	recent := time.Now().Add(time.Hour).Truncate(time.Second)
	require.NoError(tb, os.Chtimes(name+".yml", recent, recent))
	return recent
}

func countingNavigationRead(name string, reads *int) func() ([]byte, error) {
	return func() ([]byte, error) {
		*reads++
		return readFile(getConfigPath(name))
	}
}

func TestExternalNavigationEditIsObservedWithoutRestart(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{}))
	writeNavigationFile(t, "bookmarks", "links:\n- name: Before\n  link: https://before.example\n")
	settleNavigationFile(t, "bookmarks", time.Hour)
	value, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Equal(t, "Before", value.Items[0].Name)
	require.True(t, MatchNavigationTarget("https://before.example", nil))
	before, err := navigationSnapshot("bookmarks")
	require.NoError(t, err)

	// Another program replaces the file while this process keeps running.
	writeNavigationFile(t, "bookmarks", "links:\n- name: After the edit\n  link: https://after.example\n")
	settleNavigationFile(t, "bookmarks", time.Minute)

	value, err = LoadNormalBookmarks()
	require.NoError(t, err)
	require.Equal(t, "After the edit", value.Items[0].Name)
	require.False(t, MatchNavigationTarget("https://before.example", nil))
	require.True(t, MatchNavigationTarget("https://after.example", nil))
	matches, err := SearchNavigationMatches("after", false, true, 8, nil, nil)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	after, err := navigationSnapshot("bookmarks")
	require.NoError(t, err)
	require.NotSame(t, before, after, "changed content gets a new snapshot and indexes")

	// A malformed edit is reported, and correcting it is observed again.
	writeNavigationFile(t, "bookmarks", "links: [broken")
	_, err = LoadNormalBookmarks()
	require.Error(t, err)
	writeNavigationFile(t, "bookmarks", "links:\n- name: Corrected\n  link: https://corrected.example\n")
	value, err = LoadNormalBookmarks()
	require.NoError(t, err)
	require.Equal(t, "Corrected", value.Items[0].Name)
}

func TestSettledNavigationFileIsServedFromMemory(t *testing.T) {
	prepareBookmarkHotPath(t)
	writeNavigationFile(t, "bookmarks", "links:\n- name: Settled\n  link: https://settled.example\n")
	settleNavigationFile(t, "bookmarks", time.Hour)
	reads := 0
	read := countingNavigationRead("bookmarks", &reads)
	first, err := readFileCached("bookmarks", read)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		again, readErr := readFileCached("bookmarks", read)
		require.NoError(t, readErr)
		require.True(t, sameCachedBytes(first, again))
	}
	require.Equal(t, 1, reads, "an unchanged settled file is read once")
	one, err := navigationSnapshot("bookmarks")
	require.NoError(t, err)
	two, err := navigationSnapshot("bookmarks")
	require.NoError(t, err)
	require.Same(t, one, two)

	// The same content written again changes the stamp, not the snapshot.
	writeNavigationFile(t, "bookmarks", "links:\n- name: Settled\n  link: https://settled.example\n")
	settleNavigationFile(t, "bookmarks", time.Minute)
	again, err := readFileCached("bookmarks", read)
	require.NoError(t, err)
	require.Equal(t, 2, reads)
	require.True(t, sameCachedBytes(first, again), "unchanged bytes keep the cached slice")
	three, err := navigationSnapshot("bookmarks")
	require.NoError(t, err)
	require.Same(t, one, three)
	_, err = readFileCached("bookmarks", read)
	require.NoError(t, err)
	require.Equal(t, 2, reads, "the refreshed stamp is trusted again")
}

// Filesystems with coarse timestamps can record two edits at the same time.
func TestSameSizeEditWithinTimestampStepIsObserved(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{}))
	writeNavigationFile(t, "bookmarks", "links:\n- name: Alpha\n  link: https://same.example/a\n")
	stamp := unsettleNavigationFile(t, "bookmarks")
	value, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Equal(t, "Alpha", value.Items[0].Name)
	first, err := navigationSnapshot("bookmarks")
	require.NoError(t, err)
	again, err := navigationSnapshot("bookmarks")
	require.NoError(t, err)
	require.Same(t, first, again, "rereading unchanged bytes keeps the snapshot")

	before, err := os.Stat("bookmarks.yml")
	require.NoError(t, err)
	writeNavigationFile(t, "bookmarks", "links:\n- name: Bravo\n  link: https://same.example/b\n")
	require.NoError(t, os.Chtimes("bookmarks.yml", stamp, stamp))
	after, err := os.Stat("bookmarks.yml")
	require.NoError(t, err)
	require.True(t, after.ModTime().Equal(before.ModTime()), "the edit keeps the recorded time")
	require.Equal(t, before.Size(), after.Size(), "the edit keeps the size")

	value, err = LoadNormalBookmarks()
	require.NoError(t, err)
	require.Equal(t, "Bravo", value.Items[0].Name)
}

func TestSizeChangeWithPreservedTimestampIsObserved(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveNormalBookmarks(model.Bookmarks{}))
	writeNavigationFile(t, "apps", "links:\n- name: Short\n  link: https://short.example\n")
	settled := time.Now().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, os.Chtimes("apps.yml", settled, settled))
	value, err := LoadFavoriteBookmarks()
	require.NoError(t, err)
	require.Equal(t, "Short", value.Items[0].Name)

	writeNavigationFile(t, "apps", "links:\n- name: A longer name\n  link: https://longer.example\n")
	require.NoError(t, os.Chtimes("apps.yml", settled, settled))
	value, err = LoadFavoriteBookmarks()
	require.NoError(t, err)
	require.Equal(t, "A longer name", value.Items[0].Name)
}

func TestRemovedNavigationFileIsNotServedFromMemory(t *testing.T) {
	prepareBookmarkHotPath(t)
	writeNavigationFile(t, "bookmarks", "links:\n- name: Removed later\n  link: https://removed.example\n")
	settleNavigationFile(t, "bookmarks", time.Hour)
	reads := 0
	read := countingNavigationRead("bookmarks", &reads)
	_, err := readFileCached("bookmarks", read)
	require.NoError(t, err)
	_, err = ReadExistingVisibleNormalBookmarks(true)
	require.NoError(t, err)

	require.NoError(t, os.Remove("bookmarks.yml"))
	raw, err := readFileCached("bookmarks", read)
	require.Error(t, err)
	require.Nil(t, raw)
	_, err = ReadExistingVisibleNormalBookmarks(true)
	require.Error(t, err)
	_, statErr := os.Stat("bookmarks.yml")
	require.True(t, os.IsNotExist(statErr), "a failed read must not create example data")
}

func TestOnlyNavigationCollectionsAreCachedByName(t *testing.T) {
	prepareBookmarkHotPath(t)
	reads := 0
	read := func() ([]byte, error) { reads++; return []byte("Title: uncached\n"), nil }
	for i := 0; i < 2; i++ {
		raw, err := readFileCached("config", read)
		require.NoError(t, err)
		require.Equal(t, "Title: uncached\n", string(raw))
	}
	require.Equal(t, 2, reads, "config.yml is observed through readFreshAppConfig, not this cache")
}

func TestInvalidatedNavigationCacheForgetsItsStamp(t *testing.T) {
	prepareBookmarkHotPath(t)
	for _, name := range []string{"apps", "bookmarks"} {
		writeNavigationFile(t, name, "links: []\n")
		settleNavigationFile(t, name, time.Hour)
		reads := 0
		read := countingNavigationRead(name, &reads)
		_, err := readFileCached(name, read)
		require.NoError(t, err)
		invalidateFileCache(name)
		fileCacheMu.RLock()
		cached, stamp, parsed := navigationCacheSlots(name)
		raw, known, snapshot := *cached, stamp.known, *parsed
		fileCacheMu.RUnlock()
		require.Nil(t, raw)
		require.False(t, known)
		require.Nil(t, snapshot)
		_, err = readFileCached(name, read)
		require.NoError(t, err)
		require.Equal(t, 2, reads)
	}
}

// A reloaded collection passes the same private-content check as the first read.
func TestReloadedNavigationRespectsPrivateContentPolicy(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{}))
	writeNavigationFile(t, "bookmarks", "links:\n- name: Public\n  link: https://public.example\n")
	settleNavigationFile(t, "bookmarks", time.Hour)
	requireLoginForPrivateContent(t)
	_, err := LoadNormalBookmarks()
	require.NoError(t, err)

	writeNavigationFile(t, "bookmarks", "links:\n- name: Public\n  link: https://public.example\n- name: Added by hand\n  link: https://private.example\n  private: true\n")
	settleNavigationFile(t, "bookmarks", time.Minute)
	_, err = LoadNormalBookmarks()
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.False(t, MatchNavigationTargetFor("https://private.example", true, nil))
	require.False(t, MatchNavigationTargetFor("https://public.example", true, nil))

	writeNavigationFile(t, "bookmarks", "links:\n- name: Public\n  link: https://public.example\n")
	settleNavigationFile(t, "bookmarks", 30*time.Second)
	value, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Len(t, value.Items, 1)
}

func TestConcurrentReadersObserveExternalEdits(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{}))
	contents := []string{
		"links:\n- name: First\n  link: https://first.example\n",
		"links:\n- name: Second version\n  link: https://second.example\n",
	}
	// Replace the file atomically, as editors that write a temporary file and
	// rename it do, so that every reader sees one complete version.
	replace := func(content string) {
		require.NoError(t, os.WriteFile("bookmarks.next", []byte(content), 0600))
		require.NoError(t, os.Rename("bookmarks.next", "bookmarks.yml"))
	}
	replace(contents[0])
	stop := make(chan struct{})
	var readers sync.WaitGroup
	failures := make(chan string, 8)
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				value, err := LoadNormalBookmarks()
				observed := "read error"
				if err == nil && len(value.Items) == 1 {
					observed = value.Items[0].Name
				}
				if observed != "First" && observed != "Second version" {
					select {
					case failures <- observed:
					default:
					}
					return
				}
			}
		}()
	}
	for i := 1; i <= 20; i++ {
		replace(contents[i%2])
	}
	close(stop)
	readers.Wait()
	select {
	case name := <-failures:
		t.Fatalf("a reader observed something other than a complete version: %q", name)
	default:
	}
	value, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Equal(t, "First", value.Items[0].Name)
}
