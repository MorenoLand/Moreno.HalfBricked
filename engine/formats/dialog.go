package formats

import (
	"encoding/xml"
	"fmt"
	"io"
)

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
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
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
