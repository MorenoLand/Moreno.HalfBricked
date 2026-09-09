package scripting

import (
	"testing"
)

type testHost struct {
	idleCalls int
	waitCalls int
	marked    bool
}

func (h *testHost) Call(name string, args []Value) (CallResult, error) {
	switch name {
	case "WaitInit":
		h.waitCalls++
		return CallResult{}, nil
	case "IsWaitComplete":
		if h.idleCalls >= 2 {
			return CallResult{Values: []Value{1}}, nil
		}
		return CallResult{Values: []Value{0}}, nil
	case "Idle":
		h.idleCalls++
		return CallResult{Yield: true}, nil
	case "Mark":
		h.marked = true
		return CallResult{}, nil
	default:
		return CallResult{}, nil
	}
}

func TestRuntimeResumesLuaFunctionAfterHostYield(t *testing.T) {
	host := &testHost{}
	runtime, err := New(`
function Wait(amount)
  WaitInit(amount)
  while IsWaitComplete() == 0 do
    Idle()
  end
end
Wait(50)
Mark()
`, host, []string{"WaitInit", "IsWaitComplete", "Idle", "Mark"})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	if runtime.Status() != StatusSuspended || host.idleCalls != 1 || host.marked {
		t.Fatalf("first step status=%v idle=%d marked=%t", runtime.Status(), host.idleCalls, host.marked)
	}
	if err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	if runtime.Status() != StatusSuspended || host.idleCalls != 2 || host.marked {
		t.Fatalf("second step status=%v idle=%d marked=%t", runtime.Status(), host.idleCalls, host.marked)
	}
	if err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	if runtime.Status() != StatusComplete || host.waitCalls != 1 || !host.marked {
		t.Fatalf("final step status=%v waits=%d marked=%t", runtime.Status(), host.waitCalls, host.marked)
	}
}
