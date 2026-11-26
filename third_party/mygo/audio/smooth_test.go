package audio

import (
	"testing"
	"time"
)

// TestSmoothHeardClockThroughBurst checks that heard clock advances smoothly between DEEP_BUFFER bursts and never jumps backward when a burst refills.
func TestSmoothHeardClockThroughBurst(t *testing.T) {
	mu.Lock()
	defer func() {
		held, mixed, unheard = 0, 0, 0
		heardLive, heardSlow = false, false
		mu.Unlock()
	}()

	held = sampleRate / 4 // 250ms = 12000 frames
	mixed = held + 4800
	unheard = held
	heardLive = false

	t0 := time.Unix(1000, 0)
	updateHeardClockAt(t0, true)
	base := smoothHeardLocked(t0)
	if base != 4800 {
		t.Fatalf("initial smoothed heard = %d, want 4800", base)
	}

	// Simulate UI polling every 16ms while audio write blocks for 80ms.
	prev := base
	for ms := 16; ms <= 64; ms += 16 {
		got := smoothHeardLocked(t0.Add(time.Duration(ms) * time.Millisecond))
		if got <= prev {
			t.Fatalf("at +%dms: smoothed heard %d did not advance past %d", ms, got, prev)
		}
		prev = got
	}

	// Simulate an 8x 10ms burst refilling at +80ms.
	burstT := t0.Add(80 * time.Millisecond)
	for i := range 8 {
		mixed += sampleRate / 100
		updateHeardClockAt(burstT.Add(time.Duration(i*50)*time.Microsecond), true)
		got := smoothHeardLocked(burstT.Add(time.Duration(i*50) * time.Microsecond))
		if got < prev {
			t.Fatalf("burst chunk %d stepped backward: got %d < prev %d", i, got, prev)
		}
		prev = got
	}
	// After burst completes (+3840 frames), clock should sit right around 4800 + 3840 = 8640.
	if diff := prev - 8640; diff < -100 || diff > 100 {
		t.Fatalf("post-burst heard = %d, want ~8640", prev)
	}
}
