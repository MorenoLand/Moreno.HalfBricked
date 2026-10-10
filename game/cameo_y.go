package game

// scriptCameoY is the value the script call GetCameoY returns: the native closure
// 0x0013f5f8 (1.2.5; v7 0x000dcbf0) pushes FUN_0012a8dc(1) (v7 FUN_000c7a9c), the top of the
// on-screen dialogue panel expressed in 320-high script units:
//
//	((H - 0.5*y - 0.5*y) - px(32) + px(1)) * 320 / H
//
// with H the surface height in pixels, y the second component of the first
// frontend-variable pair ONSCREENDIALOG_TEXT_BOX_SIZE_VAR (480, 65; flags
// CONVERT_SIZE_X|CONVERT_SIZE_Y_KAR, so y = 65/320*H pixels) and
// px(v) = v/320 * ((2/3) / (H/W)) * H the CONVERT_SIZE_Y conversion with W the
// surface width. For the port's 480x320 logical surface this is 224.
func scriptCameoY(width, height float32) float32 {
	if width <= 0 || height <= 0 {
		return 0
	}
	const (
		half    = float32(0.5)
		virtual = float32(320)
		boxY    = float32(65)
	)
	boxHeight := boxY / virtual * height
	ratio := height / width
	px := func(value float32) float32 {
		return value / virtual * (float32(2.0/3.0) / ratio) * height
	}
	top := (height - boxHeight*half) - boxHeight*half
	return ((top - px(32)) + px(1)) * virtual / height
}
