package engine

import (
	"bytes"
	"io"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/perfmark"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

type SoundSource interface {
	Open(path string) (io.ReadCloser, error)
}

type SoundSystem struct {
	source       SoundSource
	context      *audio.Context
	music        *audio.Player
	musicEnabled bool
	cache        map[string][]byte
	pcm          map[string][]byte // decoded short sounds
	pcmBytes     int64
	// OnPlay, when set, is told about every one-shot sound that starts (not music or looping sounds): the online host
	// uses it to pass the sounds on to its guests.
	OnPlay func(path string, volume float64)
}

const (
	maxCachedPCMBytes = 6 << 20   // one sound
	maxCachedPCMTotal = 192 << 20 // all of them
)

func NewSoundSystem(source SoundSource) *SoundSystem {
	return newSoundSystem(source, true)
}

func NewSilentSoundSystem(source SoundSource) *SoundSystem {
	return newSoundSystem(source, false)
}

func newSoundSystem(source SoundSource, musicEnabled bool) *SoundSystem {
	system := &SoundSystem{source: source, musicEnabled: musicEnabled, cache: map[string][]byte{}}
	if source == nil {
		return system
	}
	_ = system.SetMusic("audio/music/sound/Music_Menu.ogg", 532640)
	return system
}
func (s *SoundSystem) SetMusic(path string, loopPointSamples int64) error {
	if s == nil || s.source == nil {
		return nil
	}
	data, err := s.bytes(path)
	if err != nil {
		return err
	}
	decodeStart := time.Now()
	stream, err := vorbis.DecodeF32(bytes.NewReader(data))
	perfmark.Since("sound decode "+path, decodeStart)
	if err != nil {
		return err
	}
	if s.context == nil {
		s.context = audio.NewContext(stream.SampleRate())
	}
	introBytes, loopBytes := musicLoopBounds(loopPointSamples, stream.Length())
	player, err := s.context.NewPlayerF32(audio.NewInfiniteLoopWithIntroF32(stream, introBytes, loopBytes))
	if err != nil {
		return err
	}
	player.SetVolume(.35)
	old := s.music
	s.music = player
	if old != nil {
		_ = old.Close()
	}
	if s.musicEnabled {
		player.Play()
	}
	return nil
}

func (s *SoundSystem) Play(path string, volume float64) {
	_, _ = s.PlayTracked(path, volume, -1)
}
func (s *SoundSystem) PlayTracked(path string, volume float64, loopPointSamples int64) (*audio.Player, error) {
	if s == nil || s.context == nil {
		return nil, io.ErrClosedPipe
	}
	if s.OnPlay != nil && loopPointSamples < 0 {
		s.OnPlay(path, volume)
	}
	// Short sounds are decoded once and replayed from PCM: decoding the whole Ogg for every shot of a fast weapon
	// was a frame-time spike each time (long music streams are still decoded per play, they are rare).
	var stream io.ReadSeeker
	var streamLength int64
	if pcm, ok := s.pcm[path]; ok {
		stream, streamLength = bytes.NewReader(pcm), int64(len(pcm))
	} else {
		data, err := s.bytes(path)
		if err != nil {
			return nil, err
		}
		decodeStart := time.Now()
		decoded, err := vorbis.DecodeF32(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer perfmark.Since("sound decode "+path, decodeStart)
		streamLength = decoded.Length()
		stream = decoded
		if streamLength > 0 && streamLength <= maxCachedPCMBytes && s.pcmBytes+streamLength <= maxCachedPCMTotal {
			if pcm, err := io.ReadAll(decoded); err == nil && int64(len(pcm)) == streamLength {
				if s.pcm == nil {
					s.pcm = map[string][]byte{}
				}
				s.pcm[path] = pcm
				s.pcmBytes += streamLength
				stream = bytes.NewReader(pcm)
			} else {
				// the stream was consumed: decode again for this play
				if again, err := vorbis.DecodeF32(bytes.NewReader(data)); err == nil {
					stream = again
				}
			}
		}
	}
	var source io.Reader = stream
	if loopPointSamples >= 0 {
		intro, length := musicLoopBounds(loopPointSamples, streamLength)
		source = audio.NewInfiniteLoopWithIntroF32(stream, intro, length)
	}
	player, err := s.context.NewPlayerF32(source)
	if err != nil {
		return nil, err
	}
	player.SetVolume(volume)
	player.Play()
	return player, nil
}

// Warm decodes a short sound into the PCM cache without playing it, so its first play costs nothing.
func (s *SoundSystem) Warm(path string) {
	if s == nil || s.source == nil || s.context == nil {
		return
	}
	if _, ok := s.pcm[path]; ok {
		return
	}
	data, err := s.bytes(path)
	if err != nil {
		return
	}
	warmStart := time.Now()
	defer perfmark.Since("sound warm "+path, warmStart)
	decoded, err := vorbis.DecodeF32(bytes.NewReader(data))
	if err != nil {
		return
	}
	length := decoded.Length()
	if length <= 0 || length > maxCachedPCMBytes || s.pcmBytes+length > maxCachedPCMTotal {
		return
	}
	pcm, err := io.ReadAll(decoded)
	if err != nil || int64(len(pcm)) != length {
		return
	}
	if s.pcm == nil {
		s.pcm = map[string][]byte{}
	}
	s.pcm[path] = pcm
	s.pcmBytes += length
}

// DecodeShort decodes a short sound to PCM without touching any cache, so it can run on a worker goroutine while the
// game loop keeps going (the data source only has to be safe to read from several goroutines, as the pack is). It
// reports false when the sound is too long to cache or does not decode.
func (s *SoundSystem) DecodeShort(path string) ([]byte, bool) {
	if s == nil || s.source == nil {
		return nil, false
	}
	reader, err := s.source.Open(path)
	if err != nil {
		return nil, false
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return nil, false
	}
	decoded, err := vorbis.DecodeF32(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	length := decoded.Length()
	if length <= 0 || length > maxCachedPCMBytes {
		return nil, false
	}
	pcm, err := io.ReadAll(decoded)
	if err != nil || int64(len(pcm)) != length {
		return nil, false
	}
	return pcm, true
}

// HasShort reports whether a short sound is already in the PCM cache.
func (s *SoundSystem) HasShort(path string) bool {
	if s == nil {
		return false
	}
	_, ok := s.pcm[path]
	return ok
}

// StoreShort puts a sound decoded by DecodeShort into the PCM cache. Call it from the game loop's goroutine (the cache
// is not locked), like Warm.
func (s *SoundSystem) StoreShort(path string, pcm []byte) {
	if s == nil || s.context == nil {
		return
	}
	if _, ok := s.pcm[path]; ok || s.pcmBytes+int64(len(pcm)) > maxCachedPCMTotal {
		return
	}
	if s.pcm == nil {
		s.pcm = map[string][]byte{}
	}
	s.pcm[path] = pcm
	s.pcmBytes += int64(len(pcm))
}

func (s *SoundSystem) MusicEnabled(enabled bool) {
	if s == nil {
		return
	}
	s.musicEnabled = enabled
	if s.music == nil {
		return
	}
	if enabled {
		s.music.Play()
	} else {
		s.music.Pause()
	}
}
func musicLoopBounds(loopPointSamples, streamLength int64) (int64, int64) {
	introBytes := loopPointSamples * 8
	if loopPointSamples <= 0 || introBytes >= streamLength {
		return 0, streamLength
	}
	return introBytes, streamLength - introBytes
}

func (s *SoundSystem) Close() error {
	if s == nil || s.music == nil {
		return nil
	}
	return s.music.Close()
}

func (s *SoundSystem) bytes(path string) ([]byte, error) {
	if s == nil || s.source == nil {
		return nil, io.ErrClosedPipe
	}
	if data, ok := s.cache[path]; ok {
		return data, nil
	}
	reader, err := s.source.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	s.cache[path] = data
	return data, nil
}
