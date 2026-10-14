package weapons

type NativeRNG struct{ state [6]uint32 }

func NewNativeRNG() NativeRNG {
	return NativeRNG{state: [6]uint32{0xdeadbeef, 0, 0x6c078965, 0x5d588b65, 0x00269ec3, 0}}
}

// NewStaticNativeRNG is the 1.2.5 global generator object (GOT 0x008e5ec4 -> 0x008ecef8) that the static initialiser at
// 0x00274c98 sets up: state 0, multiplier 0x5d588b656c078965, increment 0x269ec3. The zombie constructor (FUN_000ffa44),
// the pickup effects and the survival boss hook (FUN_000998ac) draw from it; the scene's own generator is NewNativeRNG.
func NewStaticNativeRNG() NativeRNG {
	return NativeRNG{state: [6]uint32{0, 0, 0x6c078965, 0x5d588b65, 0x00269ec3, 0}}
}

func (r *NativeRNG) Bounded(bound uint32) uint32 {
	product := uint64(r.state[2]) * uint64(r.state[0])
	low := uint32(product)
	carry := uint32((uint64(r.state[4]) + uint64(low)) >> 32)
	value := r.state[5] + r.state[2]*r.state[1] + r.state[0]*r.state[3] + uint32(product>>32) + carry
	r.state[0] = r.state[4] + low
	r.state[1] = value
	if bound-1 < 0xfffffffe {
		return uint32(uint64(value) * uint64(bound) >> 32)
	}
	return value
}
