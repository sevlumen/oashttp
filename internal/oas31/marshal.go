package oas31

import "encoding/json"

func Marshal(document *Document) ([]byte, error) { return json.Marshal(document) }
func (d *Document) Clone() (*Document, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var clone Document
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}
