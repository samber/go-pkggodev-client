// Package api is the ogen-generated pkg.go.dev API client.
package api

//go:generate wget -O openapi.yaml https://pkg.go.dev/v1beta/openapi.yaml
//go:generate bash -c "jq '.components.schemas.PaginatedResponse.properties.items.items = {} | .components.schemas.PaginatedResponse.properties.items.nullable = true' openapi.yaml > openapi.tmp && mv openapi.tmp openapi.yaml"
//go:generate go run github.com/ogen-go/ogen/cmd/ogen@latest --target . --package api --clean openapi.yaml
