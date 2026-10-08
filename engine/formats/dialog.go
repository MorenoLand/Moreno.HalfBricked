package formats

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"unicode/utf8"
)

var windows1252High = [32]rune{
	0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
	0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
}

// utf8OrWindows1252 returns data unchanged when valid UTF-8; the shipped dialog
// files declare UTF-8 but contain Windows-1252 quote bytes.
func utf8OrWindows1252(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	out := make([]rune, len(data))
	for i, b := range data {
		if b >= 0x80 && b < 0xA0 {
			out[i] = windows1252High[b-0x80]
		} else {
			out[i] = rune(b)
		}
	}
	return []byte(string(out))
}

type Conversation struct {
	Name     string   `json:"name"`
	Speeches []Speech `json:"speeches"`
}

type Speech struct {
	Cameo int      `json:"cameo"`
	Text  []string `json:"text"`
}

func ParseDialog(reader io.Reader) ([]Conversation, error) {
	var document struct {
		Conversations []struct {
			Name     string `xml:"name,attr"`
			Speeches []struct {
				Cameo int `xml:"cameo,attr"`
				Text  []struct {
					Value string `xml:"para,attr"`
				} `xml:"text"`
			} `xml:"speech"`
		} `xml:"conversation"`
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err := xml.NewDecoder(bytes.NewReader(utf8OrWindows1252(data))).Decode(&document); err != nil {
		return nil, err
	}
	result := make([]Conversation, 0, len(document.Conversations))
	for index, item := range document.Conversations {
		if item.Name == "" {
			return nil, fmt.Errorf("conversation %d has no name", index)
		}
		conversation := Conversation{Name: item.Name, Speeches: make([]Speech, 0, len(item.Speeches))}
		for _, source := range item.Speeches {
			speech := Speech{Cameo: source.Cameo, Text: make([]string, 0, len(source.Text))}
			for _, text := range source.Text {
				speech.Text = append(speech.Text, text.Value)
			}
			conversation.Speeches = append(conversation.Speeches, speech)
		}
		result = append(result, conversation)
	}
	return result, nil
}
