// Package adapters lists the source adapters this build supports.
package adapters

import (
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/greenhouse"
)

// UserAgent identifies RoleScout to sources.
const UserAgent = "RoleScout/0.1 (self-hosted job tracker; +https://github.com/0furkancolak/rolescout)"

// Registry returns every adapter. Adding an adapter requires an access review
// documented in docs/sources/.
func Registry() ingest.Registry {
	return ingest.Registry{
		"greenhouse": greenhouse.New(UserAgent),
	}
}
