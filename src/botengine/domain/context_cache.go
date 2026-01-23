package domain

import (
	"context"
	"time"
)

// ContextCacheEntry represents a cached context reference for AI providers.
// This is used to store references to provider-side caches (e.g., Gemini's CachedContent).
type ContextCacheEntry struct {
	// Name is the provider-assigned identifier for the cached content
	Name string `json:"name"`
	// ExpiresAt is when this cache entry should be considered invalid
	ExpiresAt time.Time `json:"expires_at"`
	// Model is the AI model this cache was created for
	Model string `json:"model,omitempty"`
}

// ContextCacheStore defines the contract for storing AI context cache references.
// Implementations can be in-memory (default) or distributed (Valkey).
type ContextCacheStore interface {
	// Get retrieves a cache entry by its fingerprint key.
	// Returns nil if not found or expired.
	Get(ctx context.Context, fingerprint string) (*ContextCacheEntry, error)

	// Save stores a cache entry with the given fingerprint.
	// The TTL should match or be slightly less than the provider's cache TTL.
	Save(ctx context.Context, fingerprint string, entry *ContextCacheEntry, ttl time.Duration) error

	// Delete removes a cache entry.
	Delete(ctx context.Context, fingerprint string) error

	// Cleanup removes all expired entries. Called periodically.
	Cleanup(ctx context.Context) error
}
