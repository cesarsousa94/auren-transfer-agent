package mediahub

import "encoding/json"

func marshalMap(values map[string]any) ([]byte, error) {
	return json.Marshal(values)
}

func unmarshalBytes(data []byte, output any) error {
	return json.Unmarshal(data, output)
}
