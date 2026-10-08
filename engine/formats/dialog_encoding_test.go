package formats

import (
	"strings"
	"testing"
)

func TestParseDialogWindows1252Quote(t *testing.T) {
	src := "<?xml version=\"1.0\" encoding=\"utf-8\"?><chat_file><conversation name=\"a\"><speech cameo=\"0\"><text para=\"farm!\x94 it\x92s\"/></speech></conversation></chat_file>"
	got, err := ParseDialog(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if want := "farm!\u201d it\u2019s"; got[0].Speeches[0].Text[0] != want {
		t.Fatalf("got %q want %q", got[0].Speeches[0].Text[0], want)
	}
}
