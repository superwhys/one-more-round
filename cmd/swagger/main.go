// Package main is the swagger generation entry point. Run it through
// `make swagger` (or `go generate ./cmd/swagger`) so the checked-in docs stay in
// sync with the route annotations of api/.
package main

//go:generate swag init --dir ../../ --generalInfo api/api.go --output ./docs --parseDependency --parseInternal --parseGoList true

func main() {}
