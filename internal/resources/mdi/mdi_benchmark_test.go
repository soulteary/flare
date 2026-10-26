package mdi

import (
	"github.com/soulteary/flare/config/define"
	"testing"
)

func BenchmarkIconCacheHit(b *testing.B) {
	originalFlags, theme, color := define.AppFlags, define.ThemeCurrent, define.ThemePrimaryColor
	b.Cleanup(func() { define.AppFlags = originalFlags; define.ThemeCurrent = theme; define.ThemePrimaryColor = color })
	for _, inline := range []bool{false, true} {
		mode := "external"
		if inline {
			mode = "inline"
		}
		for _, parallel := range []bool{false, true} {
			name := mode + "/serial"
			if parallel {
				name = mode + "/parallel"
			}
			b.Run(name, func(b *testing.B) {
				define.AppFlags.EnableMinimumRequest = inline
				define.ThemeCurrent = "blackboard"
				define.ThemePrimaryColor = "#fffdea"
				if err := Init(); err != nil {
					b.Fatal(err)
				}
				if GetIconByName("serverNetwork") == "" {
					b.Fatal("icon missing")
				}
				b.ReportAllocs()
				b.ResetTimer()
				if parallel {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							GetIconByName("serverNetwork")
						}
					})
				} else {
					for range b.N {
						GetIconByName("serverNetwork")
					}
				}
			})
		}
	}
}
