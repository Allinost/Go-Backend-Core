package logger

import (
	"bytes"
	"sync"
	"testing"
)

func BenchmarkLogger_WriteSequential(b *testing.B) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().Int("n", i).Msg("benchmark log")
	}
}

func BenchmarkLogger_WriteParallel(b *testing.B) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var n int
		for pb.Next() {
			l.Info().Int("n", n).Msg("parallel benchmark")
			n++
		}
	})
}

func BenchmarkLogger_ConcurrentGoroutines(b *testing.B) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for j := 0; j < 100; j++ {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				l.Info().Int("j", k).Msg("concurrent")
			}(j)
		}
		wg.Wait()
	}
}

func BenchmarkLogger_DisabledLevel(b *testing.B) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "fatal", Format: "json"}, &buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Debug().Msg("should be filtered")
	}
}

func BenchmarkLogger_TextFormat(b *testing.B) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "text"}, &buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().Str("key", "value").Msg("text benchmark")
	}
}
