package dolthub

import "strings"

// ShortOperationID returns the terminal ID of a qualified job or operation
// resource name. Bare IDs and empty values are preserved.
func ShortOperationID(id string) string {
	if i := strings.LastIndexByte(id, '/'); i >= 0 && i < len(id)-1 {
		return id[i+1:]
	}
	return id
}

// ForDisplay returns a copy with the UUID as its ID, leaving the API value intact.
func (o Operation) ForDisplay() Operation {
	o.ID = ShortOperationID(o.ID)
	return o
}

// ForDisplay shortens the ID while preserving the usable polling URL.
func (r OperationRef) ForDisplay() OperationRef {
	r.ID = ShortOperationID(r.ID)
	return r
}
