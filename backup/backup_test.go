package backup

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/gobwas/glob"
)

func BenchmarkRegex(b *testing.B) {
	re := regexp.MustCompile("node_modules")
	files := [][]byte{
		[]byte("some/path/node_modules"),
		[]byte("some/path/not_modules"),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.Match(files[i%2])
	}
}

func BenchmarkContains(b *testing.B) {
	node := []byte("node_modules")
	files := [][]byte{
		[]byte("some/path/node_modules"),
		[]byte("some/path/not_modules"),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes.Contains(files[i%2], node)
	}
}

func BenchmarkGlob(b *testing.B) {
	g := glob.MustCompile("node_modules")
	files := [][]byte{
		[]byte("some/path/node_modules"),
		[]byte("some/path/not_modules"),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Match(string(files[i%2]))
	}
}
