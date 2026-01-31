package workspace

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Deduplication(t *testing.T) {
	// 1. Test Local Deduplication (Fallback)
	t.Run("LocalFallback", func(t *testing.T) {
		m := &Manager{
			messageDedup: sync.Map{},
		}

		msgID := "test-msg-local"
		sender := "user1"
		text := "hola"

		// First attempt: should succeed
		success := m.tryLockMessage("chan1", msgID, sender, text)
		assert.True(t, success, "First attempt should acquire lock")

		// Second attempt (Same ID): should fail
		success = m.tryLockMessage("chan1", msgID, sender, text)
		assert.False(t, success, "Second attempt with same ID should be blocked")

		// Third attempt (Different ID but same content): should fail (Content Dedup)
		success = m.tryLockMessage("chan1", "new-id", sender, text)
		assert.False(t, success, "Third attempt with same content but different ID should be blocked")

		// Fourth attempt (Different content): should succeed
		success = m.tryLockMessage("chan1", "another-id", sender, "otro texto")
		assert.True(t, success, "Different content should acquire its own lock")
	})

	// 2. Test Valkey Logic (Distributed)
	t.Run("ValkeyIntegration", func(t *testing.T) {
		cfg := valkey.Config{
			Address:   "localhost:6379",
			KeyPrefix: "test:dedup",
		}

		vk, err := valkey.NewClient(cfg)
		if err != nil {
			t.Skip("Valkey not available for integration test:", err)
		}
		defer vk.Close()

		m := &Manager{
			valkeyClient: vk,
		}

		msgID := "test-msg-valkey-" + time.Now().Format("150405.000")
		sender := "user-v"
		text := "test-v"
		ctx := context.Background()

		// Ensure keys are clean in the test namespace
		// Layer 1 key: d:i:<msgID>
		idKey := vk.Key("d:i:" + msgID)
		_ = vk.Inner().Do(ctx, vk.Inner().B().Del().Key(idKey).Build()).Error()

		// First attempt: should succeed
		success := m.tryLockMessage("chan1", msgID, sender, text)
		assert.True(t, success, "First attempt should acquire Valkey lock")

		// Second attempt: should fail
		success = m.tryLockMessage("chan1", msgID, sender, text)
		assert.False(t, success, "Second attempt should be blocked by Valkey NX")

		// Verify key exists in Valkey and has no value (empty string)
		exists, err := vk.Inner().Do(ctx, vk.Inner().B().Exists().Key(idKey).Build()).AsInt64()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists, "ID Key should exist in Valkey")

		// valkey-go v1.0.x uses ToString() for ValkeyResult
		val, _ := vk.Inner().Do(ctx, vk.Inner().B().Get().Key(idKey).Build()).ToString()
		assert.Equal(t, "", val, "Valkey lock value should be empty to save memory")
	})
}

func TestManager_Normalization(t *testing.T) {
	m := &Manager{
		messageDedup: sync.Map{},
	}

	msgID1 := "id-1"
	msgID2 := "id-2" // Diferentes IDs
	text := "Hola"

	// Escenario: El mismo usuario manda "Hola" desde dos identidades distintas (JID y LID)
	jidSender := "51999999999:1@s.whatsapp.net"
	lidSender := "51999999999@lid"

	// 1. Mensaje llega por JID
	success := m.tryLockMessage("chan1", msgID1, jidSender, text)
	assert.True(t, success, "Primer mensaje debe pasar")

	// 2. Mensaje llega por LID (con ID distinto pero mismo contenido)
	success = m.tryLockMessage("chan1", msgID2, lidSender, text)
	assert.False(t, success, "Segundo mensaje DEBE BLOQUEARSE aunque el remitente parezca distinto (gracias a la normalización)")
}
