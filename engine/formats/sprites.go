package formats

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type SpriteAnimation struct {
	Name       string
	Texture    string
	Frames     int
	Angles     int
	FPS        float64
	Loop       bool
	AnimLength int
}

type SpriteDefinition struct {
	Name           string
	Animations     map[string]SpriteAnimation
	AnimationOrder []string
}

type SpriteCatalog map[string]SpriteDefinition

type spriteXML struct {
	Sprites []spriteDefinitionXML `xml:"Sprite"`
}

type spriteDefinitionXML struct {
	Name  string          `xml:"name,attr"`
	Anims []spriteAnimXML `xml:"Anim"`
}

type spriteAnimXML struct {
	Name       string `xml:"name,attr"`
	Texture    string `xml:"texture,attr"`
	Frames     string `xml:"numFrames,attr"`
	Angles     string `xml:"numAngles,attr"`
	FPS        string `xml:"fps,attr"`
	Loop       string `xml:"loop,attr"`
	AnimLength string `xml:"animLength,attr"`
}

func ParseSprites(reader io.Reader) (SpriteCatalog, error) {
	var document spriteXML
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	data = stripSpriteComments(data)
	if err := xml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	result := make(SpriteCatalog, len(document.Sprites))
	for spriteIndex, source := range document.Sprites {
		name := strings.TrimSpace(source.Name)
		if name == "" {
			return nil, fmt.Errorf("sprite %d has no name", spriteIndex)
		}
		key := strings.ToLower(name)
		definition := result[key]
		if definition.Name == "" {
			definition = SpriteDefinition{Name: name, Animations: map[string]SpriteAnimation{}}
		}
		for animIndex, item := range source.Anims {
			animName := strings.TrimSpace(item.Name)
			texture := strings.TrimSpace(item.Texture)
			if animName == "" || texture == "" {
				return nil, fmt.Errorf("sprite %q animation %d is missing name or texture", name, animIndex)
			}
			frames, err := spriteInt(item.Frames, 1, "numFrames", name, animName)
			if err != nil {
				return nil, err
			}
			angles, err := spriteInt(item.Angles, 1, "numAngles", name, animName)
			if err != nil {
				return nil, err
			}
			fps, err := spriteFloat(item.FPS, 8, "fps", name, animName)
			if err != nil {
				return nil, err
			}
			animLength, err := spriteInt(item.AnimLength, frames, "animLength", name, animName)
			if err != nil {
				return nil, err
			}
			loop, err := spriteBool(item.Loop, true, "loop", name, animName)
			if err != nil {
				return nil, err
			}
			animationKey := strings.ToLower(animName)
			if _, exists := definition.Animations[animationKey]; !exists {
				definition.AnimationOrder = append(definition.AnimationOrder, animationKey)
			}
			definition.Animations[animationKey] = SpriteAnimation{Name: animName, Texture: texture, Frames: frames, Angles: angles, FPS: fps, Loop: loop, AnimLength: animLength}
		}
		result[key] = definition
	}
	return result, nil
}

func stripSpriteComments(data []byte) []byte {
	for {
		start := bytes.Index(data, []byte("<!--"))
		if start < 0 {
			return data
		}
		relativeEnd := bytes.Index(data[start+4:], []byte("-->"))
		if relativeEnd < 0 {
			return data[:start]
		}
		end := start + 4 + relativeEnd + 3
		data = append(data[:start], data[end:]...)
	}
}

func (catalog SpriteCatalog) Find(name string) (SpriteDefinition, bool) {
	definition, ok := catalog[strings.ToLower(strings.TrimSpace(name))]
	return definition, ok
}

func (definition SpriteDefinition) Animation(name string) (SpriteAnimation, bool) {
	animation, ok := definition.Animations[strings.ToLower(strings.TrimSpace(name))]
	return animation, ok
}

func (definition SpriteDefinition) AnimationByIndex(index int) (SpriteAnimation, bool) {
	if index < 0 || index >= len(definition.AnimationOrder) {
		return SpriteAnimation{}, false
	}
	animation, ok := definition.Animations[definition.AnimationOrder[index]]
	return animation, ok
}

func spriteInt(value string, fallback int, field, sprite, animation string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		if err == nil {
			err = fmt.Errorf("must be positive")
		}
		return 0, fmt.Errorf("sprite %q animation %q %s %q: %w", sprite, animation, field, value, err)
	}
	return parsed, nil
}

func spriteFloat(value string, fallback float64, field, sprite, animation string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 {
		if err == nil {
			err = fmt.Errorf("must be positive")
		}
		return 0, fmt.Errorf("sprite %q animation %q %s %q: %w", sprite, animation, field, value, err)
	}
	return parsed, nil
}

func spriteBool(value string, fallback bool, field, sprite, animation string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, fmt.Errorf("sprite %q animation %q %s %q: want 0/1 or true/false", sprite, animation, field, value)
	}
}
