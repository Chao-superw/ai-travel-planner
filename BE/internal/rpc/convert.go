package rpc

import (
	"encoding/json"
	"errors"
)

func convert[T any](v any) (*T, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, errors.New("RPC encode failed")
	}
	var out T
	if e = json.Unmarshal(b, &out); e != nil {
		return nil, errors.New("RPC decode failed")
	}
	return &out, nil
}
