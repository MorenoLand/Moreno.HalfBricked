package scripting

import "testing"

type nullHost struct{}

func (nullHost) Call(string, []Value) (CallResult, error) { return CallResult{}, nil }

func TestSuccessorSeesPreviousScriptGlobals(t *testing.T) {
	first, err := New("function Wait(n) return n end\nanswer = Wait(7)", nullHost{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Step(); err != nil || !first.Done() {
		t.Fatalf("first script: %v done=%v", err, first.Done())
	}
	second, err := first.Successor("if Wait(answer) ~= 7 then error('globals lost') end", nullHost{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := second.Step(); err != nil || second.Status() != StatusComplete {
		t.Fatalf("successor: %v status=%v", err, second.Status())
	}
	if first.state != nil {
		t.Fatal("previous runtime still owns the Lua state")
	}
}

type countingHost struct{ polls int }

func (h *countingHost) Call(name string, args []Value) (CallResult, error) {
	h.polls++
	if h.polls > 20000 {
		return CallResult{Values: []Value{0}}, nil
	}
	return CallResult{Values: []Value{1}}, nil
}

// A loop that polls a host function without Idle() must yield instead of
// freezing, and every poll must still return its own value.
func TestBusyPollingLoopYieldsAndKeepsResults(t *testing.T) {
	host := &countingHost{}
	runtime, err := New("local n = 0\nwhile Poll() == 1 do n = n + 1 end\nresult = n", host, []string{"Poll"})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	steps := 0
	for !runtime.Done() && steps < 50 {
		if err := runtime.Step(); err != nil {
			t.Fatal(err)
		}
		steps++
	}
	if !runtime.Done() {
		t.Fatal("polling loop never finished")
	}
	if steps < 2 {
		t.Fatalf("loop finished in %d step(s); it should have yielded", steps)
	}
	if got := runtime.state.GetGlobal("result"); got.String() != "20000" {
		t.Fatalf("poll results were lost across yields: result=%v", got)
	}
}
