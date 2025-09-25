package clipboard

// ReadAll returns an empty string because the stub clipboard implementation
// is intentionally inert in environments without system clipboard access.
func ReadAll() (string, error) {
	return "", nil
}

// WriteAll is a no-op for the stub clipboard implementation.
func WriteAll(string) error {
	return nil
}

// Unsupported reports whether clipboard operations are unsupported. The stub
// always returns true to signal callers that system clipboards are not
// available.
func Unsupported(error) bool {
	return true
}
