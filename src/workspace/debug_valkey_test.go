package workspace

import (
	"context"
	"fmt"
	"testing"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValkeySyntax_Debug(t *testing.T) {
	// 1. Setup
	cfg := valkey.Config{Address: "localhost:6379", KeyPrefix: "debug"}
	vk, err := valkey.NewClient(cfg)
	if err != nil {
		t.Skip("No valkey")
	}
	defer vk.Close()
	ctx := context.Background()
	key := vk.Key("syntax_check")

	// 2. Clean & Write
	vk.Inner().Do(ctx, vk.Inner().B().Del().Key(key).Build())
	err = vk.Inner().Do(ctx, vk.Inner().B().Zadd().Key(key).ScoreMember().ScoreMember(100, "item1").Build()).Error()
	require.NoError(t, err)

	// 3. Test ZRANGE 0 0 WITHSCORES (Index based)
	// Try Method A: Min/Max
	cmdA := vk.Inner().B().Zrange().Key(key).Min("0").Max("0").Withscores().Build()
	resA, _ := vk.Inner().Do(ctx, cmdA).AsStrSlice()
	fmt.Printf("METHOD A (Min/Max 0-0): %v (len %d)\n", resA, len(resA))

	// Try Method D: ZRANGEBYSCORE (Infinite range for 'next')
	cmdD := vk.Inner().B().Zrangebyscore().Key(key).Min("-inf").Max("+inf").Withscores().Limit(0, 1).Build()
	resD, errD := vk.Inner().Do(ctx, cmdD).AsStrSlice()
	fmt.Printf("METHOD D (ByScore): %v (err: %v)\n", resD, errD)

	// Assert at least one works
	working := len(resD) >= 2
	assert.True(t, working, "Method D should return [item1, 100]")
}
