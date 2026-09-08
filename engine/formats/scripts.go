package formats

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type ScriptCommand struct {
	Name string
	Args []string
	Line int
}
type Script struct{ Commands []ScriptCommand }

func ParseScript(reader io.Reader) (Script, error) {
	var result Script
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripScriptComment(scanner.Text()))
		if line == "" {
			continue
		}
		open := strings.IndexByte(line, '(')
		if open <= 0 || !strings.HasSuffix(line, ")") {
			continue
		}
		name := strings.TrimSpace(line[:open])
		if !scriptIdentifier(name) || scriptKeyword(name) {
			continue
		}
		args, err := scriptArgs(line[open+1 : len(line)-1])
		if err != nil {
			return Script{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		result.Commands = append(result.Commands, ScriptCommand{Name: name, Args: args, Line: lineNumber})
	}
	if err := scanner.Err(); err != nil {
		return Script{}, err
	}
	return result, nil
}

func scriptKeyword(value string) bool {
	switch value {
	case "if", "elseif", "while", "for", "function", "return", "repeat", "until":
		return true
	default:
		return false
	}
}

func stripScriptComment(line string) string {
	quoted := false
	escaped := false
	for i := 0; i+1 < len(line); i++ {
		if line[i] == '\\' && quoted && !escaped {
			escaped = true
			continue
		}
		if line[i] == '"' && !escaped {
			quoted = !quoted
		}
		escaped = false
		if !quoted && line[i] == '-' && line[i+1] == '-' {
			return line[:i]
		}
	}
	return line
}

func scriptIdentifier(value string) bool {
	if value == "" || (value[0] < 'A' || value[0] > 'Z') && (value[0] < 'a' || value[0] > 'z') && value[0] != '_' {
		return false
	}
	for _, r := range value[1:] {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func scriptArgs(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var result []string
	start := 0
	quoted := false
	escaped := false
	for i, r := range value {
		if r == '\\' && quoted && !escaped {
			escaped = true
			continue
		}
		if r == '"' && !escaped {
			quoted = !quoted
		}
		escaped = false
		if r == ',' && !quoted {
			arg, err := scriptArg(value[start:i])
			if err != nil {
				return nil, err
			}
			result = append(result, arg)
			start = i + 1
		}
	}
	if quoted {
		return nil, fmt.Errorf("unterminated string")
	}
	arg, err := scriptArg(value[start:])
	if err != nil {
		return nil, err
	}
	return append(result, arg), nil
}

func scriptArg(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return "", err
		}
		return decoded, nil
	}
	return value, nil
}
