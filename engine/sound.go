package engine

import (
	"bytes"
	"io"

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
}

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
	stream, err := vorbis.DecodeF32(bytes.NewReader(data))
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
	data, err := s.bytes(path)
	if err != nil {
		return nil, err
	}
	stream, err := vorbis.DecodeF32(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var source io.Reader = stream
	if loopPointSamples >= 0 {
		intro, length := musicLoopBounds(loopPointSamples, stream.Length())
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
