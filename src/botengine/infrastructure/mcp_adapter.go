package infrastructure

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

type mcpClientEntry struct {
	client   *client.Client
	config   domainMCP.MCPServer
	lastUsed time.Time
}

// MCPProviderAdapter implementa IMCPProvider usando mcp-go.
type MCPProviderAdapter struct {
	clients sync.Map // serverID -> *mcpClientEntry
}

func NewMCPProviderAdapter() *MCPProviderAdapter {
	a := &MCPProviderAdapter{}
	go a.startIdleClientCleaner()
	return a
}

func (a *MCPProviderAdapter) ListTools(ctx context.Context, server domainMCP.MCPServer) ([]domainMCP.Tool, error) {
	c, err := a.getOrConnectClient(ctx, server)
	if err != nil {
		return nil, err
	}

	res, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	var tools []domainMCP.Tool
	for _, t := range res.Tools {
		tools = append(tools, domainMCP.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return tools, nil
}

func (a *MCPProviderAdapter) CallTool(ctx context.Context, server domainMCP.MCPServer, toolName string, args map[string]interface{}) (domainMCP.CallToolResult, error) {
	c, err := a.getOrConnectClient(ctx, server)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolName
	callReq.Params.Arguments = args

	res, err := c.CallTool(ctx, callReq)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

	var result domainMCP.CallToolResult
	result.IsError = res.IsError
	for _, content := range res.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			result.Content = append(result.Content, domainMCP.CallToolContent{
				Type: "text",
				Text: textContent.Text,
			})
		}
	}

	return result, nil
}

func (a *MCPProviderAdapter) Validate(ctx context.Context, server domainMCP.MCPServer, fullHandshake bool) ([]domainMCP.Tool, error) {
	// Check basic reachability first (status 200/etc)
	if err := a.checkAvailability(ctx, server); err != nil {
		return nil, err
	}

	if !fullHandshake {
		return nil, nil
	}

	// Full connection and tools listing
	mcpClient, err := a.createClient(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	defer mcpClient.Close()

	if err := a.initializeClient(ctx, mcpClient, server.Name); err != nil {
		return nil, err
	}

	res, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	var tools []domainMCP.Tool
	for _, t := range res.Tools {
		tools = append(tools, domainMCP.Tool{
			Name: t.Name, Description: t.Description, InputSchema: t.InputSchema,
		})
	}
	return tools, nil
}

func (a *MCPProviderAdapter) Shutdown() {
	logrus.Info("[MCPAdapter] Shutting down connections...")
	a.clients.Range(func(key, value interface{}) bool {
		if entry, ok := value.(*mcpClientEntry); ok {
			entry.client.Close()
		}
		return true
	})
}

// === Lógica interna de red (Adaptador) ===

func (a *MCPProviderAdapter) getOrConnectClient(ctx context.Context, server domainMCP.MCPServer) (*client.Client, error) {
	if val, ok := a.clients.Load(server.ID); ok {
		entry := val.(*mcpClientEntry)
		if entry.config.URL == server.URL && reflect.DeepEqual(entry.config.Headers, server.Headers) {
			entry.lastUsed = time.Now()
			return entry.client, nil
		}
		entry.client.Close()
		a.clients.Delete(server.ID)
	}

	mcpClient, err := a.createClient(ctx, server)
	if err != nil {
		return nil, err
	}

	if err := a.initializeClient(ctx, mcpClient, server.Name); err != nil {
		mcpClient.Close()
		return nil, err
	}

	entry := &mcpClientEntry{
		client:   mcpClient,
		config:   server,
		lastUsed: time.Now(),
	}
	a.clients.Store(server.ID, entry)
	return mcpClient, nil
}

func (a *MCPProviderAdapter) createClient(ctx context.Context, server domainMCP.MCPServer) (*client.Client, error) {
	logrus.Infof("[MCPAdapter] Connecting to %s (%s)", server.Name, server.Type)
	var mcpClient *client.Client
	var err error

	switch server.Type {
	case domainMCP.ConnTypeHTTP:
		var opts []transport.StreamableHTTPCOption
		if len(server.Headers) > 0 {
			logrus.Debugf("[MCPAdapter] Applying %d custom headers to HTTP client", len(server.Headers))
			opts = append(opts, transport.WithHTTPHeaders(server.Headers))
		}
		mcpClient, err = client.NewStreamableHttpClient(server.URL, opts...)
	default: // SSE
		var opts []transport.ClientOption
		if len(server.Headers) > 0 {
			logrus.Debugf("[MCPAdapter] Applying %d custom headers to SSE client", len(server.Headers))
			opts = append(opts, client.WithHeaders(server.Headers))
		}
		mcpClient, err = client.NewSSEMCPClient(server.URL, opts...)
	}

	if err != nil {
		return nil, err
	}
	if err := mcpClient.Start(ctx); err != nil {
		return nil, err
	}
	return mcpClient, nil
}

func (a *MCPProviderAdapter) initializeClient(ctx context.Context, mcpClient *client.Client, name string) error {
	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{Name: "az-wap-bot", Version: "1.0.0"}

	var initErr error
	for i := 0; i < 5; i++ {
		_, initErr = mcpClient.Initialize(ctx, req)
		if initErr == nil {
			return nil
		}
		if strings.Contains(strings.ToLower(initErr.Error()), "session") {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	return initErr
}

func (a *MCPProviderAdapter) checkAvailability(ctx context.Context, server domainMCP.MCPServer) error {
	if server.URL == "" {
		return fmt.Errorf("URL missing")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range server.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned status code %d", resp.StatusCode)
	}

	return nil
}

func (a *MCPProviderAdapter) startIdleClientCleaner() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		now := time.Now()
		a.clients.Range(func(key, value interface{}) bool {
			entry := value.(*mcpClientEntry)
			if now.Sub(entry.lastUsed) > 10*time.Minute {
				logrus.Infof("[MCPAdapter] Closing idle connection for %s", entry.config.Name)
				entry.client.Close()
				a.clients.Delete(key)
			}
			return true
		})
	}
}
