package cpix

// EscapeODataID escapes an OData string literal used inside single quotes.
//
// CPI OData endpoints use single quotes to delimit key values, so single quotes
// inside the value must be escaped by doubling them.
func EscapeODataID(id string) string {
	return escapeODataID(id)
}
