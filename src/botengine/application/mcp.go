package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/botengine/infrastructure"
	"github.com/AzielCF/az-wap/botengine/repository"
	coreconfig "github.com/AzielCF/az-wap/core/config"
	domainHealth "github.com/AzielCF/az-wap/domains/health"
	"github.com/AzielCF/az-wap/pkg/crypto"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type mcpService struct {
	repo     domainMCP.IMCPRepository
	provider domainMCP.IMCPProvider
	health   domainHealth.IHealthUsecase
}

func NewMCPService(db *gorm.DB) domainMCP.IMCPUsecase {
	if err := crypto.SetEncryptionKey(coreconfig.Global.Security.SecretKey); err != nil {
		logrus.WithError(err).Error("[MCP] failed key set")
	}
	repo := repository.NewMCPGormRepository(db)
	provider := infrastructure.NewMCPProviderAdapter() // Adaptador de infraestructura

	return &mcpService{
		repo:     repo,
		provider: provider,
	}
}

func NewMCPServiceWithDeps(repo domainMCP.IMCPRepository, provider domainMCP.IMCPProvider) domainMCP.IMCPUsecase {
	return &mcpService{
		repo:     repo,
		provider: provider,
	}
}

func (s *mcpService) ensureRepo() error {
	if s.repo == nil {
		return fmt.Errorf("storage not ready")
	}
	return nil
}

// === Server Management ===

func (s *mcpService) AddServer(ctx context.Context, server domainMCP.MCPServer) (domainMCP.MCPServer, error) {
	if err := s.ensureRepo(); err != nil {
		return server, err
	}
	if server.ID == "" {
		server.ID = uuid.NewString()
	}

	// Validación de HTTPS
	if server.Type == domainMCP.ConnTypeSSE {
		if os.Getenv("MCP_ALLOW_INSECURE_HTTP") != "true" && !strings.HasPrefix(server.URL, "https://") {
			return server, fmt.Errorf("HTTPS required for SSE")
		}
	}

	tools, err := s.provider.Validate(ctx, server, !server.IsTemplate)
	s.reportHealth(ctx, server.ID, err)
	if err != nil {
		return server, pkgError.ValidationError(fmt.Sprintf("validation failed: %v", err))
	}

	if len(tools) > 0 {
		server.Tools = tools
	}

	return server, s.repo.AddServer(ctx, server)
}

func (s *mcpService) ListServers(ctx context.Context) ([]domainMCP.MCPServer, error) {
	if err := s.ensureRepo(); err != nil {
		return nil, err
	}
	return s.repo.ListServers(ctx)
}

func (s *mcpService) GetServer(ctx context.Context, id string) (domainMCP.MCPServer, error) {
	if err := s.ensureRepo(); err != nil {
		return domainMCP.MCPServer{}, err
	}
	return s.repo.GetServer(ctx, id)
}

func (s *mcpService) UpdateServer(ctx context.Context, id string, server domainMCP.MCPServer) (domainMCP.MCPServer, error) {
	if err := s.ensureRepo(); err != nil {
		return server, err
	}

	tools, err := s.provider.Validate(ctx, server, !server.IsTemplate)
	s.reportHealth(ctx, id, err)
	if err != nil {
		return server, pkgError.ValidationError(fmt.Sprintf("validation failed: %v", err))
	}

	if len(tools) > 0 {
		server.Tools = tools
	}

	return server, s.repo.UpdateServer(ctx, id, server)
}

func (s *mcpService) DeleteServer(ctx context.Context, id string) error {
	if err := s.ensureRepo(); err != nil {
		return err
	}
	return s.repo.DeleteServer(ctx, id)
}

// === Tools Interaction ===

func (s *mcpService) ListTools(ctx context.Context, id string) ([]domainMCP.Tool, error) {
	srv, err := s.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}

	tools, err := s.provider.ListTools(ctx, srv)
	if err != nil {
		return nil, err
	}

	if len(tools) > 0 {
		_ = s.repo.UpdateServerTools(ctx, id, tools)
	}
	return tools, nil
}

func (s *mcpService) CallTool(ctx context.Context, botID string, req domainMCP.CallToolRequest) (domainMCP.CallToolResult, error) {
	srv, err := s.GetServer(ctx, req.ServerID)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

	// Inyectar cabeceras personalizadas del bot
	if cfg, err := s.repo.GetBotMCPConfig(ctx, botID, req.ServerID); err == nil && cfg.ConfigJSON != "" {
		var bc domainMCP.BotMCPConfigJSON
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &bc); err == nil {
			if srv.Headers == nil {
				srv.Headers = make(map[string]string)
			}
			for k, v := range bc.CustomHeaders {
				if dec, err := crypto.Decrypt(v); err == nil {
					srv.Headers[k] = dec
				} else {
					srv.Headers[k] = v
				}
			}
		}
	}

	res, err := s.provider.CallTool(ctx, srv, req.ToolName, req.Arguments)
	if err != nil {
		s.reportHealth(ctx, req.ServerID, err)
		return domainMCP.CallToolResult{}, err
	}

	s.reportHealth(ctx, req.ServerID, nil)
	return res, nil
}

// === Bot Specific ===

func (s *mcpService) GetBotTools(ctx context.Context, botID string) ([]domainMCP.Tool, error) {
	serverIDs, err := s.repo.GetBotServerIDs(ctx, botID)
	if err != nil {
		return nil, err
	}

	var all []domainMCP.Tool
	for _, sid := range serverIDs {
		srv, err := s.GetServer(ctx, sid)
		if err != nil {
			continue
		}

		disabled := make(map[string]bool)
		if cfg, _ := s.repo.GetBotMCPConfig(ctx, botID, sid); cfg.ConfigJSON != "" {
			var bc domainMCP.BotMCPConfigJSON
			if err := json.Unmarshal([]byte(cfg.ConfigJSON), &bc); err == nil {
				for _, t := range bc.DisabledTools {
					disabled[t] = true
				}
			}
		}

		for _, t := range srv.Tools {
			if !disabled[t.Name] {
				all = append(all, t)
			}
		}
	}
	return all, nil
}

func (s *mcpService) ListServersForBot(ctx context.Context, botID string) ([]domainMCP.MCPServer, error) {
	servers, err := s.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range servers {
		if cfg, err := s.repo.GetBotMCPConfig(ctx, botID, servers[i].ID); err == nil {
			servers[i].Enabled = cfg.Enabled
			if cfg.ConfigJSON != "" {
				var bc domainMCP.BotMCPConfigJSON
				json.Unmarshal([]byte(cfg.ConfigJSON), &bc)
				servers[i].DisabledTools = bc.DisabledTools
				servers[i].BotInstructions = bc.Instructions
				servers[i].CustomHeaders = s.decryptMap(bc.CustomHeaders)
			}
		}
	}
	return servers, nil
}

func (s *mcpService) ToggleServerForBot(ctx context.Context, botID, serverID string, enabled bool) error {
	return s.repo.ToggleServerForBot(ctx, botID, serverID, enabled)
}

func (s *mcpService) UpdateBotMCPConfig(ctx context.Context, cfg domainMCP.BotMCPConfig) error {
	// Optimization: Skip validation if headers haven't changed and it was already enabled
	doValidate := false
	if cfg.Enabled {
		existing, err := s.repo.GetBotMCPConfig(ctx, cfg.BotID, cfg.ServerID)
		if err != nil || !existing.Enabled {
			// If it wasn't enabled or we can't find it, we MUST validate
			doValidate = true
		} else {
			// Compare custom headers
			var bc domainMCP.BotMCPConfigJSON
			if err := json.Unmarshal([]byte(existing.ConfigJSON), &bc); err == nil {
				decHeaders := s.decryptMap(bc.CustomHeaders)
				if !reflect.DeepEqual(decHeaders, cfg.CustomHeaders) {
					doValidate = true
				}
			} else {
				doValidate = true
			}
		}
	}

	if doValidate {
		srv, err := s.GetServer(ctx, cfg.ServerID)
		if err == nil {
			if srv.Headers == nil {
				srv.Headers = make(map[string]string)
			}
			for k, v := range cfg.CustomHeaders {
				srv.Headers[k] = v
			}
			if _, err := s.provider.Validate(ctx, srv, true); err != nil {
				s.reportHealth(ctx, cfg.ServerID, err)
				return fmt.Errorf("bot validation failed: %w", err)
			}
			s.health.ReportSuccess(ctx, domainHealth.EntityMCP, cfg.ServerID)
		}
	}

	encHeaders := make(map[string]string)
	for k, v := range cfg.CustomHeaders {
		enc, _ := crypto.Encrypt(v)
		encHeaders[k] = enc
	}

	confJSON, _ := json.Marshal(domainMCP.BotMCPConfigJSON{
		DisabledTools: cfg.DisabledTools,
		CustomHeaders: encHeaders,
		Instructions:  cfg.Instructions,
	})

	return s.repo.SaveBotMCPConfig(ctx, cfg.BotID, cfg.ServerID, cfg.Enabled, string(confJSON))
}

func (s *mcpService) Validate(ctx context.Context, id string) error {
	srv, err := s.GetServer(ctx, id)
	if err != nil {
		return err
	}
	tools, err := s.provider.Validate(ctx, srv, !srv.IsTemplate)
	s.reportHealth(ctx, id, err)
	if err == nil && len(tools) > 0 {
		_ = s.repo.UpdateServerTools(ctx, id, tools)
	}
	return err
}

func (s *mcpService) ListBotsUsingServer(ctx context.Context, sid string) ([]string, error) {
	return s.repo.ListBotsUsingServer(ctx, sid)
}

func (s *mcpService) SetHealthUsecase(h domainHealth.IHealthUsecase) {
	s.health = h
}

func (s *mcpService) Shutdown() {
	s.provider.Shutdown()
}

// === Helpers ===

func (s *mcpService) reportHealth(ctx context.Context, id string, err error) {
	if s.health == nil {
		return
	}
	if err != nil {
		s.health.ReportFailure(ctx, domainHealth.EntityMCP, id, err.Error())
	} else {
		s.health.ReportSuccess(ctx, domainHealth.EntityMCP, id)
	}
}

func (s *mcpService) decryptMap(m map[string]string) map[string]string {
	res := make(map[string]string)
	for k, v := range m {
		if dec, err := crypto.Decrypt(v); err == nil {
			res[k] = dec
		} else {
			res[k] = v
		}
	}
	return res
}
