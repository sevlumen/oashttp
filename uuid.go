package oashttp

import "github.com/quang020102/go-osm/internal/core"

type UUID string

func ParseUUID(value string) (UUID, error) {
	normalized, err := core.NormalizeUUID(value)
	if err != nil {
		return "", err
	}
	return UUID(normalized), nil
}

func (u UUID) String() string { return string(u) }
func (u *UUID) UnmarshalText(text []byte) error {
	value, err := ParseUUID(string(text))
	if err != nil {
		return err
	}
	*u = value
	return nil
}
func (u UUID) MarshalText() ([]byte, error) { return []byte(u), nil }
