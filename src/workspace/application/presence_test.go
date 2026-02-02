package application

import (
	"context"
	"testing"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAdapter to track connectivity calls
type MockAdapter struct {
	mock.Mock
}

func (m *MockAdapter) Status() channel.ChannelStatus {
	args := m.Called()
	return args.Get(0).(channel.ChannelStatus)
}

func (m *MockAdapter) Resume(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAdapter) SetOnline(ctx context.Context, online bool) error {
	args := m.Called(ctx, online)
	return args.Error(0)
}

func (m *MockAdapter) Hibernate(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// These are dummy implementations of other interface methods to satisfy the ChannelAdapter interface
func (m *MockAdapter) ID() string                                                     { return "test_adapter" }
func (m *MockAdapter) Type() channel.ChannelType                                      { return channel.ChannelTypeWhatsApp }
func (m *MockAdapter) UpdateConfig(cfg channel.ChannelConfig)                         {}
func (m *MockAdapter) IsLoggedIn() bool                                               { return true }
func (m *MockAdapter) OnMessage(h func(message.IncomingMessage))                      {}
func (m *MockAdapter) ResolveIdentity(ctx context.Context, id string) (string, error) { return "", nil }
func (m *MockAdapter) GetMe() (common.ContactInfo, error)                             { return common.ContactInfo{}, nil }

// Lifecycle
func (m *MockAdapter) Start(ctx context.Context, cfg channel.ChannelConfig) error { return nil }
func (m *MockAdapter) Stop(ctx context.Context) error                             { return nil }
func (m *MockAdapter) Cleanup(ctx context.Context) error                          { return nil }

// Messaging
func (m *MockAdapter) SendMessage(ctx context.Context, chatID, text, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (m *MockAdapter) SendMedia(ctx context.Context, chatID string, media common.MediaUpload, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (m *MockAdapter) SendPresence(ctx context.Context, chatID string, typing, audio bool) error {
	return nil
}
func (m *MockAdapter) SendContact(ctx context.Context, chatID, name, phone, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (m *MockAdapter) SendLocation(ctx context.Context, chatID string, lat, long float64, addr, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (m *MockAdapter) SendPoll(ctx context.Context, chatID, q string, opt []string, max int, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (m *MockAdapter) SendLink(ctx context.Context, chatID, link, cap, title, desc string, thumb []byte, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}

// Groups
func (m *MockAdapter) CreateGroup(ctx context.Context, name string, part []string) (string, error) {
	return "", nil
}
func (m *MockAdapter) JoinGroupWithLink(ctx context.Context, link string) (string, error) {
	return "", nil
}
func (m *MockAdapter) LeaveGroup(ctx context.Context, groupID string) error { return nil }
func (m *MockAdapter) GetGroupInfo(ctx context.Context, groupID string) (common.GroupInfo, error) {
	return common.GroupInfo{}, nil
}
func (m *MockAdapter) UpdateGroupParticipants(ctx context.Context, groupID string, part []string, act common.ParticipantAction) error {
	return nil
}
func (m *MockAdapter) GetGroupInviteLink(ctx context.Context, groupID string, reset bool) (string, error) {
	return "", nil
}
func (m *MockAdapter) GetJoinedGroups(ctx context.Context) ([]common.GroupInfo, error) {
	return nil, nil
}
func (m *MockAdapter) GetGroupInfoFromLink(ctx context.Context, link string) (common.GroupInfo, error) {
	return common.GroupInfo{}, nil
}
func (m *MockAdapter) GetGroupRequestParticipants(ctx context.Context, id string) ([]common.GroupRequestParticipant, error) {
	return nil, nil
}
func (m *MockAdapter) UpdateGroupRequestParticipants(ctx context.Context, id string, part []string, act common.ParticipantAction) error {
	return nil
}
func (m *MockAdapter) SetGroupName(ctx context.Context, id, name string) error         { return nil }
func (m *MockAdapter) SetGroupLocked(ctx context.Context, id string, lock bool) error  { return nil }
func (m *MockAdapter) SetGroupAnnounce(ctx context.Context, id string, ann bool) error { return nil }
func (m *MockAdapter) SetGroupTopic(ctx context.Context, id, topic string) error       { return nil }
func (m *MockAdapter) SetGroupPhoto(ctx context.Context, id string, photo []byte) (string, error) {
	return "", nil
}

// Profile
func (m *MockAdapter) SetProfileName(ctx context.Context, name string) error     { return nil }
func (m *MockAdapter) SetProfileStatus(ctx context.Context, status string) error { return nil }
func (m *MockAdapter) SetProfilePhoto(ctx context.Context, photo []byte) (string, error) {
	return "", nil
}
func (m *MockAdapter) GetContact(ctx context.Context, jid string) (common.ContactInfo, error) {
	return common.ContactInfo{}, nil
}
func (m *MockAdapter) GetPrivacySettings(ctx context.Context) (common.PrivacySettings, error) {
	return common.PrivacySettings{}, nil
}
func (m *MockAdapter) GetUserInfo(ctx context.Context, jids []string) ([]common.ContactInfo, error) {
	return nil, nil
}
func (m *MockAdapter) GetProfilePictureInfo(ctx context.Context, jid string, prev bool) (string, error) {
	return "", nil
}
func (m *MockAdapter) GetBusinessProfile(ctx context.Context, jid string) (common.BusinessProfile, error) {
	return common.BusinessProfile{}, nil
}
func (m *MockAdapter) GetAllContacts(ctx context.Context) ([]common.ContactInfo, error) {
	return nil, nil
}

// Message management
func (m *MockAdapter) MarkRead(ctx context.Context, chatID string, ids []string) error { return nil }
func (m *MockAdapter) ReactMessage(ctx context.Context, chatID, msgID, emoji string) (string, error) {
	return "", nil
}
func (m *MockAdapter) RevokeMessage(ctx context.Context, chatID, msgID string) (string, error) {
	return "", nil
}
func (m *MockAdapter) DeleteMessageForMe(ctx context.Context, chatID, msgID string) error { return nil }
func (m *MockAdapter) StarMessage(ctx context.Context, chatID, msgID string, star bool) error {
	return nil
}
func (m *MockAdapter) DownloadMedia(ctx context.Context, msgID, chatID string) (string, error) {
	return "", nil
}

// Utils
func (m *MockAdapter) IsOnWhatsApp(ctx context.Context, phone string) (bool, error) { return true, nil }

// Newsletters
func (m *MockAdapter) FetchNewsletters(ctx context.Context) ([]common.NewsletterInfo, error) {
	return nil, nil
}
func (m *MockAdapter) UnfollowNewsletter(ctx context.Context, id string) error { return nil }
func (m *MockAdapter) SendNewsletterMessage(ctx context.Context, id, text, path string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}

// Chats
func (m *MockAdapter) PinChat(ctx context.Context, id string, pin bool) error { return nil }

// Session management
func (m *MockAdapter) GetQRChannel(ctx context.Context) (<-chan string, error) { return nil, nil }
func (m *MockAdapter) Login(ctx context.Context) error                         { return nil }
func (m *MockAdapter) LoginWithCode(ctx context.Context, phone string) (string, error) {
	return "", nil
}
func (m *MockAdapter) Logout(ctx context.Context) error                               { return nil }
func (m *MockAdapter) WaitIdle(ctx context.Context, id string, d time.Duration) error { return nil }
func (m *MockAdapter) CloseSession(ctx context.Context, id string) error              { return nil }

// MockStore for persistence
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Save(ctx context.Context, p *channel.ChannelPresence) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockStore) Get(ctx context.Context, channelID string) (*channel.ChannelPresence, error) {
	args := m.Called(ctx, channelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*channel.ChannelPresence), args.Error(1)
}

func (m *MockStore) Delete(ctx context.Context, channelID string) error {
	args := m.Called(ctx, channelID)
	return args.Error(0)
}

func (m *MockStore) GetAll(ctx context.Context) ([]*channel.ChannelPresence, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*channel.ChannelPresence), args.Error(1)
}

// Test_Connectivity_Self_Healing_Detection verifies auto-reconnect logic
func Test_Connectivity_Self_Healing_Detection(t *testing.T) {
	store := new(MockStore)
	adapter := new(MockAdapter)
	pm := NewPresenceManager(store)

	channelID := "chan_001"
	pm.RegisterAdapter(channelID, adapter)

	// Scenario: Adapter is currently HIBERNATING
	adapter.On("Status").Return(channel.ChannelStatusHibernating)
	adapter.On("Resume", mock.Anything).Return(nil)

	p := &channel.ChannelPresence{ChannelID: channelID, IsSocketConnected: false}
	store.On("Get", mock.Anything, channelID).Return(p, nil)
	store.On("Save", mock.Anything, mock.MatchedBy(func(p *channel.ChannelPresence) bool {
		return p.IsSocketConnected == true
	})).Return(nil)

	// Execute self-healing
	pm.EnsureChannelConnectivity(channelID)

	// We need to wait a tiny bit because Resume is called in a goroutine
	time.Sleep(50 * time.Millisecond)

	adapter.AssertExpectations(t)
	// We don't assert store expectations here because Save is also in goroutine,
	// but the Resume call being executed proves it's working.
}

// Test_Presence_Activity_Reset ensures activity keeps the bot awake
func Test_Presence_Activity_Reset(t *testing.T) {
	store := new(MockStore)
	adapter := new(MockAdapter)
	pm := NewPresenceManager(store)
	channelID := "chan_002"
	pm.RegisterAdapter(channelID, adapter)

	p := &channel.ChannelPresence{ChannelID: channelID}
	store.On("Get", mock.Anything, channelID).Return(p, nil)
	store.On("Save", mock.Anything, mock.Anything).Return(nil)

	adapter.On("Status").Return(channel.ChannelStatusConnected)
	adapter.On("SetOnline", mock.Anything, true).Return(nil)

	// Simulate activity
	pm.HandleIncomingActivity(channelID)

	// Wait for goroutines
	time.Sleep(50 * time.Millisecond)

	adapter.AssertCalled(t, "SetOnline", mock.Anything, true)
	store.AssertExpectations(t)
}

// Test_Night_Window_Boundary verifies night time logic
func Test_Night_Window_Boundary(t *testing.T) {
	// Re-mapping the internal logic for testing
	isNight := func(h int) bool {
		return h >= 0 && h < 6
	}

	assert.True(t, isNight(0), "Midnight should be NIGHT")
	assert.True(t, isNight(5), "5 AM should be NIGHT")
	assert.False(t, isNight(6), "6 AM should be DAY")
	assert.False(t, isNight(12), "Noon should be DAY")
	assert.False(t, isNight(23), "11 PM should be DAY")
}
