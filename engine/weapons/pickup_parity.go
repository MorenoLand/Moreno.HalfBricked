package weapons

import (
	"image"
	"strings"
)

type pickupNativeBinding struct {
	Type, Subtype, Cell int
	Secondary           bool
}

func PickupParityBinding(name string) (pickupNativeBinding, bool) {
	var kind int
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "p_shotgun":
		kind = 0x26
	case "p_uzi":
		kind = 0x27
	case "p_minigun":
		kind = 0x28
	case "p_sniper":
		kind = 0x29
	case "p_flamer":
		kind = 0x2a
	case "p_buzzsaw":
		kind = 0x2b
	case "p_dual_pistol":
		kind = 0x2c
	case "p_grenade":
		kind = 0x2d
	case "p_mine":
		kind = 0x2e
	case "p_bazooka":
		kind = 0x2f
	case "p_sentry":
		kind = 0x30
	case "p_cow_pat":
		kind = 0x31
	default:
		return pickupNativeBinding{}, false
	}
	subtype := kind - 0x25
	cell := subtype
	if subtype >= 8 {
		cell -= 8
	}
	return pickupNativeBinding{Type: kind, Subtype: subtype, Cell: cell, Secondary: subtype >= 8}, true
}
func pickupParityCell(bounds image.Rectangle, cell int) (image.Rectangle, bool) {
	if cell < 0 || cell >= 8 || bounds.Empty() {
		return image.Rectangle{}, false
	}
	x := bounds.Min.X + int(float32(bounds.Dx())*float32(cell)*0.125)
	end := bounds.Min.X + int(float32(bounds.Dx())*float32(cell+1)*0.125)
	return image.Rect(x, bounds.Min.Y, end, bounds.Max.Y), end > x
}
func pickupParityGeometry(x, y, width, height, elevation float32) (float32, float32, float32, float32) {
	return x, (y - height*0.375) - elevation, width, -height
}
func PickupParityVoice(prefix, symbol string) (string, bool) {
	var suffix string
	switch symbol {
	case "SFX_VO_SHOTGUN":
		suffix = "Shotgun"
	case "SFX_VO_SMG":
		suffix = "SMG"
	case "SFX_VO_MINIGUN":
		suffix = "Minigun"
	case "SFX_VO_RIFLE":
		suffix = "Rifle"
	case "SFX_VO_FLAMETHROWER":
		suffix = "Flamethrower"
	case "SFX_VO_GRENADES":
		suffix = "Grenades"
	case "SFX_VO_MINES":
		suffix = "Mines"
	case "SFX_VO_BAZOOKA":
		suffix = "Bazooka"
	case "SFX_VO_SHIELD":
		// FUN_000928f4 plays voice id 8 for the p_shield pickup; the voice symbol
		// table (_INIT_70) lists SFX_VO_SHIELD in that slot.
		suffix = "Shield"
	case "SFX_VO_DYNAMITE":
		suffix = "Dynamite"
	case "SFX_VO_PEACEMAKER":
		suffix = "Peacemaker"
	default:
		return "", false
	}
	if prefix == "" || strings.ContainsAny(prefix, "/\\\x00") {
		return "", false
	}
	return "audio/sound/sfx/" + prefix + "_" + suffix + ".ogg", true
}
