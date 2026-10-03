package main

func scoreTextGeometry(x, y, width, height, scale, frontendX, frontendY float64) (float64, float64, float64) {
	return x*frontendX - width*frontendX/2, y*frontendY - height*frontendX/2, scale * frontendX
}
