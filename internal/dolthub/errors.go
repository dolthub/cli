package dolthub

import "fmt"

// APIError describes an unsuccessful DoltHub API response without request secrets.
type APIError struct {
	Status    int
	Method    string
	Path      string
	RequestID string
	Type      string
	Title     string
	Detail    string
	Code      string
}

func (e *APIError) Error() string {
	message := e.Detail
	if message == "" {
		message = e.Title
	}
	if message == "" {
		message = "DoltHub API request failed"
	}
	if e.RequestID != "" {
		return fmt.Sprintf("%s %s: %d %s (request ID %s)", e.Method, e.Path, e.Status, message, e.RequestID)
	}
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.Status, message)
}
