package route

import (
	"strconv"
	"time"

	"github.com/oashttp/oashttp/internal/core"
)

type Constraint struct {
	JSONType string
	Format   string
	Validate func(string) error
}

var Constraints = map[string]Constraint{
	"string":   {JSONType: "string", Validate: func(string) error { return nil }},
	"uuid":     {JSONType: "string", Format: "uuid", Validate: func(s string) error { _, err := core.NormalizeUUID(s); return err }},
	"int":      {JSONType: "integer", Format: "int32", Validate: func(s string) error { _, err := strconv.ParseInt(s, 10, 32); return err }},
	"int64":    {JSONType: "integer", Format: "int64", Validate: func(s string) error { _, err := strconv.ParseInt(s, 10, 64); return err }},
	"bool":     {JSONType: "boolean", Validate: func(s string) error { _, err := strconv.ParseBool(s); return err }},
	"date":     {JSONType: "string", Format: "date", Validate: func(s string) error { _, err := time.Parse("2006-01-02", s); return err }},
	"datetime": {JSONType: "string", Format: "date-time", Validate: func(s string) error { _, err := time.Parse(time.RFC3339, s); return err }},
}
