package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
)

// ValkeyPresenceStore implements channel.PresenceStore using Valkey.
type ValkeyPresenceStore struct {
	client *valkey.Client
	prefix string
}

// NewValkeyPresenceStore creates a new ValkeyPresenceStore instance.
func NewValkeyPresenceStore(client *valkey.Client) *ValkeyPresenceStore {
	return &ValkeyPresenceStore{
		client: client,
		prefix: client.Key("presence") + ":",
	}
}

func (s *ValkeyPresenceStore) fullKey(channelID string) string {
	return s.prefix + channelID
}

// Save stores or updates the presence state of a channel.
func (s *ValkeyPresenceStore) Save(ctx context.Context, presence *channel.ChannelPresence) error {
	data, err := json.Marshal(presence)
	if err != nil {
		return fmt.Errorf("failed to marshal presence: %w", err)
	}

	cmd := s.client.Inner().B().Set().
		Key(s.fullKey(presence.ChannelID)).
		Value(string(data)).
		Build()

	if err := s.client.Inner().Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to save presence to valkey: %w", err)
	}
	return nil
}

// Get retrieves the presence state of a channel.
func (s *ValkeyPresenceStore) Get(ctx context.Context, channelID string) (*channel.ChannelPresence, error) {
	cmd := s.client.Inner().B().Get().Key(s.fullKey(channelID)).Build()
	data, err := s.client.Inner().Do(ctx, cmd).AsBytes()
	if err != nil {
		if valkey.IsNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get presence from valkey: %w", err)
	}

	var presence channel.ChannelPresence
	if err := json.Unmarshal(data, &presence); err != nil {
		return nil, fmt.Errorf("failed to unmarshal presence: %w", err)
	}
	return &presence, nil
}

// Delete removes the presence record of a channel.
func (s *ValkeyPresenceStore) Delete(ctx context.Context, channelID string) error {
	cmd := s.client.Inner().B().Del().Key(s.fullKey(channelID)).Build()
	return s.client.Inner().Do(ctx, cmd).Error()
}

// GetAll returns all registered presence states.
func (s *ValkeyPresenceStore) GetAll(ctx context.Context) ([]*channel.ChannelPresence, error) {
	var presences []*channel.ChannelPresence
	var cursor uint64

	for {
		// Use SCAN for safety
		scanCmd := s.client.Inner().B().Scan().Cursor(cursor).Match(s.prefix + "*").Count(100).Build()
		result, err := s.client.Inner().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return nil, fmt.Errorf("failed to scan presence keys: %w", err)
		}

		if len(result.Elements) > 0 {
			mgetCmd := s.client.Inner().B().Mget().Key(result.Elements...).Build()
			values, err := s.client.Inner().Do(ctx, mgetCmd).AsStrSlice()
			if err != nil {
				return nil, fmt.Errorf("failed to mget presences: %w", err)
			}

			for _, val := range values {
				if val == "" {
					continue
				}
				var p channel.ChannelPresence
				if err := json.Unmarshal([]byte(val), &p); err == nil {
					presences = append(presences, &p)
				}
			}
		}

		cursor = result.Cursor
		if cursor == 0 {
			break
		}
	}

	return presences, nil
}
