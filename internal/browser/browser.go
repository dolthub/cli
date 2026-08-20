package browser

// Browser opens a URL without exposing browser implementation details to commands.
type Browser interface{ Browse(string) error }
