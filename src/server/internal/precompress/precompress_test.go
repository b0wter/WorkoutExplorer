package precompress

import (
	"bytes"
	"io"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/andybalholm/brotli"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/workout"
)

func decompress(t *testing.T, b []byte) []byte {
	t.Helper()
	out, err := io.ReadAll(brotli.NewReader(bytes.NewReader(b)))
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return out
}

func TestBrotliRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	big := make([]byte, 2<<20)
	for i := range big {
		big[i] = byte(rng.UintN(256))
	}
	tests := []struct {
		name string
		src  []byte
	}{
		{"empty", nil},
		{"one byte", []byte{0x42}},
		{"2 MB pseudo-random", big},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br, err := Brotli(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if len(br) == 0 {
				t.Fatal("empty output is not a valid brotli stream")
			}
			if got := decompress(t, br); !bytes.Equal(got, tt.src) {
				t.Errorf("round trip differs: %d bytes in, %d bytes out", len(tt.src), len(got))
			}
		})
	}
}

// TestBrotliDataFile compresses a real Encode output (ADR-006 item 4: the
// .br decompresses to exactly the .bin).
func TestBrotliDataFile(t *testing.T) {
	const n = 3600
	w := workout.Workout{Sport: "cycling", StartTime: time.Date(2026, 6, 6, 5, 0, 0, 0, time.UTC)}
	w.Samples.Offset = make([]int32, n)
	w.Samples.HeartRate = make([]float64, n)
	w.Samples.Altitude = make([]float64, n)
	for i := range n {
		w.Samples.Offset[i] = int32(i)
		w.Samples.HeartRate[i] = float64(120 + i%40)
		w.Samples.Altitude[i] = 400 + float64(i%300)/10
	}
	bin, err := datafile.Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	br, err := Brotli(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompress(t, br), bin) {
		t.Error("decompressed data file differs from the original")
	}
	if len(br) >= len(bin) {
		t.Errorf("brotli did not compress: %d >= %d bytes", len(br), len(bin))
	}
}

// TestBrotliOneAtATime: Brotli waits while another compression holds the
// package mutex (F-03 design v1.2, Pre-compression).
func TestBrotliOneAtATime(t *testing.T) {
	mu.Lock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := Brotli([]byte("x")); err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-done:
		mu.Unlock()
		t.Fatal("Brotli ran while the mutex was held")
	case <-time.After(100 * time.Millisecond):
	}
	mu.Unlock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Brotli did not finish after the mutex was released")
	}
}
