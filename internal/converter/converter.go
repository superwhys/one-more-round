// Package converter maps between the persistence models, the domain entities
// and the HTTP DTOs. It is the only package that knows all three shapes, so
// every boundary crossing goes through one place.
package converter

// Converter holds the mapping methods. It is stateless; the struct exists so
// callers can inject and test the conversions as one unit.
type Converter struct{}

// New builds a converter.
func New() *Converter { return &Converter{} }
