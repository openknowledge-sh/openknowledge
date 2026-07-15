package okf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// ReadFileAtMost reads one local contract without allowing an oversized file
// to force an unbounded allocation.
func ReadFileAtMost(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("file byte limit must be positive")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxBytes {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", path, maxBytes)
	}
	return content, nil
}

// DecodeStrictJSON rejects duplicate object keys, unknown destination fields,
// and trailing JSON before returning a decoded external or persisted contract.
func DecodeStrictJSON(content []byte, destination any) error {
	uniqueDecoder := json.NewDecoder(bytes.NewReader(content))
	uniqueDecoder.UseNumber()
	if err := decodeUniqueJSONValue(uniqueDecoder); err != nil {
		return err
	}
	if _, err := uniqueDecoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("JSON contains trailing JSON")
		}
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("JSON contains trailing JSON")
		}
		return err
	}
	return nil
}

func decodeUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON contains a non-string object key")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("JSON contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := decodeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return fmt.Errorf("JSON contains a malformed object")
		}
	case '[':
		for decoder.More() {
			if err := decodeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return fmt.Errorf("JSON contains a malformed array")
		}
	default:
		return fmt.Errorf("JSON contains unexpected delimiter %q", delimiter)
	}
	return nil
}
