package windowx

import "testing"

func TestSumAfterTickBeyond(t *testing.T) {
	w := New(3)
	w.Add(10)        // cells=[10,0,0], now=0
	w.Tick()         // now=1, cells[1]=0
	w.Add(20)        // cells=[10,20,0]
	w.Tick()         // now=2
	w.Add(30)        // cells=[10,20,30]
	w.Tick()         // now=0, cells[0]=0
	w.Add(40)        // cells=[0,20,30] + 40 -> [40,20,30]
	if got := w.Sum(); got != 90 {
		t.Fatalf("Sum=%d want 90", got)
	}
}
