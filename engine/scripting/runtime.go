package scripting

import (
	"fmt"
	"math"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type Value interface{}

type CallResult struct {
	Values []Value
	Yield  bool
}

type Host interface {
	Call(name string, args []Value) (CallResult, error)
}

type Status uint8

const (
	StatusRunning Status = iota
	StatusSuspended
	StatusComplete
	StatusFailed
)

type Runtime struct {
	state    *lua.LState
	thread   *lua.LState
	cancel   func()
	function *lua.LFunction
	status   Status
	err      error
}

func New(source string, host Host, callbacks []string) (*Runtime, error) {
	if host == nil {
		return nil, fmt.Errorf("script host is nil")
	}
	state := lua.NewState(lua.Options{IncludeGoStackTrace: true, SkipOpenLibs: true})
	lua.OpenBase(state)
	lua.OpenTable(state)
	lua.OpenString(state)
	lua.OpenMath(state)
	lua.OpenCoroutine(state)
	for _, name := range callbacks {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		state.SetGlobal(name, state.NewFunction(hostFunction(host, name)))
	}
	function, err := state.LoadString(source)
	if err != nil {
		state.Close()
		return nil, err
	}
	thread, cancel := state.NewThread()
	return &Runtime{state: state, thread: thread, cancel: cancel, function: function}, nil
}

func (r *Runtime) Step() error {
	if r == nil {
		return fmt.Errorf("script runtime is nil")
	}
	if r.status == StatusComplete || r.status == StatusFailed {
		return r.err
	}
	status, err, _ := r.state.Resume(r.thread, r.function)
	switch status {
	case lua.ResumeYield:
		r.status = StatusSuspended
	case lua.ResumeOK:
		r.status = StatusComplete
	case lua.ResumeError:
		r.status = StatusFailed
		if err == nil {
			err = fmt.Errorf("script resumed with an error")
		}
		r.err = err
	}
	return r.err
}

func (r *Runtime) Status() Status {
	if r == nil {
		return StatusFailed
	}
	return r.status
}

func (r *Runtime) Done() bool {
	return r == nil || r.status == StatusComplete || r.status == StatusFailed
}

func (r *Runtime) Err() error {
	if r == nil {
		return fmt.Errorf("script runtime is nil")
	}
	return r.err
}

func (r *Runtime) CurrentLine() int {
	if r == nil || r.thread == nil {
		return 0
	}
	if debug, ok := r.thread.GetStack(0); ok {
		if _, err := r.thread.GetInfo("Sl", debug, lua.LNil); err != nil {
			return 0
		}
		return debug.CurrentLine
	}
	return 0
}

func (r *Runtime) Close() {
	if r == nil || r.state == nil {
		return
	}
	if r.cancel != nil {
		r.cancel()
	}
	r.state.Close()
	r.state = nil
	r.thread = nil
	r.function = nil
}

func hostFunction(host Host, name string) lua.LGFunction {
	return func(state *lua.LState) int {
		args := make([]Value, state.GetTop())
		for index := range args {
			value, err := fromLuaValue(state.Get(index + 1))
			if err != nil {
				state.RaiseError("%s: %v", name, err)
				return 0
			}
			args[index] = value
		}
		result, err := host.Call(name, args)
		if err != nil {
			state.RaiseError("%s: %v", name, err)
			return 0
		}
		values := make([]lua.LValue, len(result.Values))
		for index, value := range result.Values {
			values[index], err = toLuaValue(value)
			if err != nil {
				state.RaiseError("%s: %v", name, err)
				return 0
			}
		}
		if result.Yield {
			return state.Yield(values...)
		}
		for _, value := range values {
			state.Push(value)
		}
		return len(values)
	}
}

func fromLuaValue(value lua.LValue) (Value, error) {
	switch value := value.(type) {
	case *lua.LNilType:
		return nil, nil
	case lua.LBool:
		return bool(value), nil
	case lua.LNumber:
		return float64(value), nil
	case lua.LString:
		return string(value), nil
	case *lua.LUserData:
		return value.Value, nil
	default:
		return nil, fmt.Errorf("unsupported argument type %s", value.Type().String())
	}
}

func toLuaValue(value Value) (lua.LValue, error) {
	switch value := value.(type) {
	case nil:
		return lua.LNil, nil
	case bool:
		return lua.LBool(value), nil
	case string:
		return lua.LString(value), nil
	case int:
		return lua.LNumber(value), nil
	case int8:
		return lua.LNumber(value), nil
	case int16:
		return lua.LNumber(value), nil
	case int32:
		return lua.LNumber(value), nil
	case int64:
		return lua.LNumber(value), nil
	case uint:
		return lua.LNumber(value), nil
	case uint8:
		return lua.LNumber(value), nil
	case uint16:
		return lua.LNumber(value), nil
	case uint32:
		return lua.LNumber(value), nil
	case uint64:
		return lua.LNumber(value), nil
	case float32:
		return lua.LNumber(value), nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("invalid number %v", value)
		}
		return lua.LNumber(value), nil
	default:
		return nil, fmt.Errorf("unsupported return type %T", value)
	}
}
