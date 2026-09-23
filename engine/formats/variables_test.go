package formats

import (
	"strings"
	"testing"
)

func TestParseVariablesRetainsXMLFlags(t *testing.T) {
	variables, err := ParseVariables(strings.NewReader(`<Variables><Vec2 name="SHOPFRONT_NEW_STORY_ICON_POS_VAR" value="320,20" flags="CONVERT_POS_X|CONVERT_POS_Y"/><Vec2 name="SHOPFRONT_TEXT_BOX_INNER_POS_VAR" value="0.401,0.745" flags="NDC_POS_X|NDC_POS_Y"/></Variables>`))
	if err != nil {
		t.Fatal(err)
	}
	position, ok := variables.Vec2Value("SHOPFRONT_NEW_STORY_ICON_POS_VAR")
	flags := variables["SHOPFRONT_NEW_STORY_ICON_POS_VAR"].Flags
	if !ok || position != (Vec2{X: 320, Y: 20}) || len(flags) != 2 || flags[0] != "CONVERT_POS_X" || flags[1] != "CONVERT_POS_Y" {
		t.Fatalf("position=%#v flags=%#v", position, flags)
	}
	if got := variables["SHOPFRONT_TEXT_BOX_INNER_POS_VAR"].Flags; len(got) != 2 || got[0] != "NDC_POS_X" || got[1] != "NDC_POS_Y" {
		t.Fatalf("NDC flags = %#v", got)
	}
}
