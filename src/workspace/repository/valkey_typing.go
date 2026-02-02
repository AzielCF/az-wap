package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
)

const typingTTL = 20 * time.Second

// ValkeyTypingStore implements channel.TypingStore using Valkey.
// It uses automatic TTL for self-cleanup of stale typing states.
type ValkeyTypingStore struct {
	client *valkey.Client
	prefix string
}

// NewValkeyTypingStore creates a new ValkeyTypingStore instance.
func NewValkeyTypingStore(client *valkey.Client) *ValkeyTypingStore {
	return &ValkeyTypingStore{
		client: client,
		prefix: client.Key("typing") + ":",
	}
}

func (s *ValkeyTypingStore) fullKey(channelID, chatID string) string {
	return s.prefix + channelID + ":" + chatID
}

// Update registers or updates the typing state.
func (s *ValkeyTypingStore) Update(ctx context.Context, channelID, chatID string, isTyping bool, media channel.TypingMedia) error {
	key := s.fullKey(channelID, chatID)

	if !isTyping {
		cmd := s.client.Inner().B().Del().Key(key).Build()
		return s.client.Inner().Do(ctx, cmd).Error()
	}

	state := channel.TypingState{
		ChannelID: channelID,
		ChatID:    chatID,
		Media:     media,
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	cmd := s.client.Inner().B().Set().
		Key(key).
		Value(string(data)).
		Ex(typingTTL).
		Build()

	return s.client.Inner().Do(ctx, cmd).Error()
}

// Get retrieves the current typing state of a chat.
func (s *ValkeyTypingStore) Get(ctx context.Context, channelID, chatID string) (*channel.TypingState, error) {
	cmd := s.client.Inner().B().Get().Key(s.fullKey(channelID, chatID)).Build()
	data, err := s.client.Inner().Do(ctx, cmd).AsBytes()
	if err != nil {
		if valkey.IsNil(err) {
			return nil, nil
		}
		return nil, err
	}

	var state channel.TypingState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// GetAll returns all active and non-expired typing states.
func (s *ValkeyTypingStore) GetAll(ctx context.Context) ([]channel.TypingState, error) {
	var states []channel.TypingState
	var cursor uint64

	for {
		scanCmd := s.client.Inner().B().Scan().Cursor(cursor).Match(s.prefix + "*").Count(100).Build()
		result, err := s.client.Inner().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return nil, err
		}

		if len(result.Elements) > 0 {
			mgetCmd := s.client.Inner().B().Mget().Key(result.Elements...).Build()
			values, err := s.client.Inner().Do(ctx, mgetCmd).AsStrSlice()
			if err != nil {
				return nil, err
			}

			for _, val := range values {
				if val == "" {
					continue
				}
				var st channel.TypingState
				if err := json.Unmarshal([]byte(val), &st); err == nil {
					states = append(states, st)
				}
			}
		}

		cursor = result.Cursor
		if cursor == 0 {
			break
		}
	}

	return states, nil
}
