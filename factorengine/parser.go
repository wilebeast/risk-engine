package factorengine

import (
	"fmt"
	"strings"
)

func ParseFactorRef(raw string) (FactorRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return FactorRef{}, ValidationError{Code: ErrFactorRefInvalid, Message: "empty factor reference"}
	}

	parts := strings.Split(raw, ".")
	for _, part := range parts {
		if part == "" {
			return FactorRef{}, ValidationError{Code: ErrFactorRefInvalid, Message: fmt.Sprintf("invalid factor reference: %q", raw)}
		}
	}

	ref := FactorRef{
		FactorCode:   parts[0],
		PathSegments: parts[1:],
	}
	ref.IsDefaultValueRef = len(ref.PathSegments) == 0 || (len(ref.PathSegments) == 1 && ref.PathSegments[0] == "value")
	return ref, nil
}

func ParseResponsePath(raw string) (ResponsePath, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ResponsePath{}, ValidationError{Code: ErrResponsePathInvalid, Message: "empty response path"}
	}
	parts := strings.Split(raw, ".")
	for _, part := range parts {
		if part == "" {
			return ResponsePath{}, ValidationError{Code: ErrResponsePathInvalid, Message: fmt.Sprintf("invalid response path: %q", raw)}
		}
	}
	return ResponsePath{Raw: raw, Segments: parts}, nil
}
