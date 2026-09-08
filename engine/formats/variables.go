package formats

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Vec2 struct{ X, Y float64 }
type FrontendVariable struct {
	Kind, Name, Value string
	Vec2              Vec2
	Float             float64
}
type FrontendVariables map[string]FrontendVariable

func ParseVariables(reader io.Reader) (FrontendVariables, error) {
	result := FrontendVariables{}
	decoder := xml.NewDecoder(reader)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || (start.Name.Local != "Vec2" && start.Name.Local != "Float" && start.Name.Local != "String") {
			continue
		}
		var item struct {
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		}
		if err := decoder.DecodeElement(&item, &start); err != nil {
			return nil, err
		}
		if item.Name == "" {
			return nil, fmt.Errorf("variable without a name")
		}
		variable := FrontendVariable{Kind: start.Name.Local, Name: item.Name, Value: item.Value}
		switch variable.Kind {
		case "Vec2":
			parts := strings.Split(item.Value, ",")
			if len(parts) != 2 {
				return nil, fmt.Errorf("variable %q: invalid Vec2 %q", item.Name, item.Value)
			}
			variable.Vec2.X, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			if err != nil {
				return nil, fmt.Errorf("variable %q: invalid x: %w", item.Name, err)
			}
			variable.Vec2.Y, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil {
				return nil, fmt.Errorf("variable %q: invalid y: %w", item.Name, err)
			}
		case "Float":
			variable.Float, err = strconv.ParseFloat(strings.TrimSpace(item.Value), 64)
			if err != nil {
				return nil, fmt.Errorf("variable %q: invalid float: %w", item.Name, err)
			}
		}
		if _, exists := result[item.Name]; exists {
			return nil, fmt.Errorf("duplicate variable %q", item.Name)
		}
		result[item.Name] = variable
	}
}

func (v FrontendVariables) Vec2Value(name string) (Vec2, bool) {
	item, ok := v[name]
	return item.Vec2, ok && item.Kind == "Vec2"
}

func (v FrontendVariables) FloatValue(name string) (float64, bool) {
	item, ok := v[name]
	return item.Float, ok && item.Kind == "Float"
}
