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
	source  SoundSource
	context *audio.Context
	music   *audio.Player
	cache   map[string][]byte
}

func NewSoundSystem(source SoundSource) *SoundSystem {
	system := &SoundSystem{source: source, cache: map[string][]byte{}}
	if source == nil {
		return system
	}
	data, err := system.bytes("audio/music/sound/Music_Menu.ogg")
	if err != nil {
		return system
	}
	stream, err := vorbis.DecodeF32(bytes.NewReader(data))
	if err != nil {
		return system
	}
	system.context = audio.NewContext(stream.SampleRate())
	system.music, err = system.context.NewPlayerF32(audio.NewInfiniteLoopF32(stream, stream.Length()))
	if err != nil {
		system.context = nil
		return system
	}
	system.music.SetVolume(.35)
	system.music.Play()
	return system
}

func (s *SoundSystem) Play(path string, volume float64) {
	if s == nil || s.context == nil {
		return
	}
	data, err := s.bytes(path)
	if err != nil {
		return
	}
	stream, err := vorbis.Decode(s.context, bytes.NewReader(data))
	if err != nil {
		return
	}
	player, err := s.context.NewPlayer(stream)
	if err != nil {
		return
	}
	player.SetVolume(volume)
	player.Play()
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
