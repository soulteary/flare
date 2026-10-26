package define

import (
	"sync"
	"testing"
)

func TestConcurrentThemeSnapshotsStayCoherent(t *testing.T) {
	before, _ := ThemeSnapshot()
	defer SetTheme(before)
	SetTheme("blackboard")
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			for j := 0; j < 100; j++ {
				if i%2 == 0 {
					SetTheme([]string{"blackboard", "cloud", "pumpkin"}[j%3])
				}
				name, color := ThemeSnapshot()
				if color != GetThemePrimaryColor(name) {
					t.Errorf("incoherent snapshot %q %q", name, color)
					return
				}
				_ = GetAppBodyStyle()
			}
		}(i)
	}
	group.Wait()
}
