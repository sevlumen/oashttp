package core

import "fmt"

func NormalizeUUID(value string) (string, error) {
	if len(value) != 36 {
		return "", fmt.Errorf("uuid must be exactly 36 characters")
	}
	b := []byte(value)
	for i, c := range b {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return "", fmt.Errorf("uuid must contain hyphens at positions 8, 13, 18, and 23")
			}
		default:
			switch {
			case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
			case c >= 'A' && c <= 'F':
				b[i] = c + ('a' - 'A')
			default:
				return "", fmt.Errorf("uuid contains a non-hexadecimal character")
			}
		}
	}
	return string(b), nil
}
