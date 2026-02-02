package valkey

import (
	"context"
	"fmt"
	"strings"
	"time"

	valkeylib "github.com/valkey-io/valkey-go"
)

const (
	// DefaultConnectTimeout is the maximum time to wait for initial connection
	DefaultConnectTimeout = 5 * time.Second
)

// Config holds the configuration for creating a Valkey client
type Config struct {
	Address        string
	Password       string
	DB             int
	KeyPrefix      string
	ConnectTimeout time.Duration // Optional, defaults to DefaultConnectTimeout
}

// Client wraps the valkey-go client with application-specific functionality.
// This struct should be created via NewClient and passed as a dependency.
type Client struct {
	inner     valkeylib.Client
	keyPrefix string
}

// NewClient creates a new Valkey client instance.
// The caller is responsible for calling Close() when done.
// Returns an error if the connection cannot be established within the timeout.
func NewClient(cfg Config) (*Client, error) {
	opts := valkeylib.ClientOption{
		InitAddress: []string{cfg.Address},
		SelectDB:    cfg.DB,
	}
	if cfg.Password != "" {
		opts.Password = cfg.Password
	}

	inner, err := valkeylib.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	// Use configured timeout or default
	timeout := cfg.ConnectTimeout
	if timeout == 0 {
		timeout = DefaultConnectTimeout
	}

	// Test connection with ping (with timeout to avoid hanging)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := inner.Do(ctx, inner.B().Ping().Build()).Error(); err != nil {
		inner.Close()
		return nil, fmt.Errorf("failed to ping valkey (timeout: %v): %w", timeout, err)
	}

	prefix := cfg.KeyPrefix
	if prefix != "" && !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}

	return &Client{
		inner:     inner,
		keyPrefix: prefix,
	}, nil
}

// Inner returns the underlying valkey-go client.
// Use this when you need direct access to the raw client.
func (c *Client) Inner() valkeylib.Client {
	return c.inner
}

// Close closes the Valkey connection.
func (c *Client) Close() {
	if c.inner != nil {
		c.inner.Close()
	}
}

// Key constructs a prefixed key from the given parts.
// Example: Key("session", "user123") -> "azwap:session:user123"
func (c *Client) Key(parts ...string) string {
	if len(parts) == 0 {
		return strings.TrimSuffix(c.keyPrefix, ":")
	}
	key := c.keyPrefix
	for i, p := range parts {
		key += p
		if i < len(parts)-1 {
			key += ":"
		}
	}
	return key
}

// KeyPrefix returns the configured key prefix.
func (c *Client) KeyPrefix() string {
	return c.keyPrefix
}

// Ping tests the connection to Valkey with a context for timeout control.
func (c *Client) Ping(ctx context.Context) error {
	return c.inner.Do(ctx, c.inner.B().Ping().Build()).Error()
}

// IsConnected tests if the connection is healthy (uses a short timeout).
func (c *Client) IsConnected() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return c.Ping(ctx) == nil
}

// IsNil checks if an error returned by the client represents a Valkey NIL response.
func IsNil(err error) bool {
	return valkeylib.IsValkeyNil(err)
}
