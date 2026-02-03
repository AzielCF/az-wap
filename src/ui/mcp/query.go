package mcp

import (
	"context"
	"fmt"
	"strconv"

	domainMessage "github.com/AzielCF/az-wap/domains/message"
	domainUser "github.com/AzielCF/az-wap/domains/user"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type QueryHandler struct {
	userService    domainUser.IUserUsecase
	messageService domainMessage.IMessageUsecase
}

func InitMcpQuery(userService domainUser.IUserUsecase, messageService domainMessage.IMessageUsecase) *QueryHandler {
	return &QueryHandler{
		userService:    userService,
		messageService: messageService,
	}
}

func (h *QueryHandler) AddQueryTools(mcpServer *server.MCPServer) {
	mcpServer.AddTool(h.toolListContacts(), h.handleListContacts)
	mcpServer.AddTool(h.toolDownloadMedia(), h.handleDownloadMedia)
}

func (h *QueryHandler) toolListContacts() mcp.Tool {
	return mcp.NewTool(
		"whatsapp_list_contacts",
		mcp.WithDescription("Retrieve all contacts available in the connected WhatsApp account."),
		mcp.WithTitleAnnotation("List Contacts"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func (h *QueryHandler) handleListContacts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = request
	resp, err := h.userService.MyListContacts(ctx, "")
	if err != nil {
		return nil, err
	}

	fallback := fmt.Sprintf("Found %d contacts", len(resp.Data))
	return mcp.NewToolResultStructured(resp, fallback), nil
}

func (h *QueryHandler) toolDownloadMedia() mcp.Tool {
	return mcp.NewTool(
		"whatsapp_download_message_media",
		mcp.WithDescription("Download media associated with a specific message and return the local file path."),
		mcp.WithTitleAnnotation("Download Message Media"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithString("message_id",
			mcp.Description("The WhatsApp message ID that contains the media."),
			mcp.Required(),
		),
		mcp.WithString("phone",
			mcp.Description("The target chat phone number or JID associated with the message."),
			mcp.Required(),
		),
	)
}

func (h *QueryHandler) handleDownloadMedia(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messageID, err := request.RequireString("message_id")
	if err != nil {
		return nil, err
	}

	phone, err := request.RequireString("phone")
	if err != nil {
		return nil, err
	}

	utils.SanitizePhone(&phone)

	req := domainMessage.DownloadMediaRequest{
		MessageID: messageID,
		Phone:     phone,
	}

	resp, err := h.messageService.DownloadMedia(ctx, req)
	if err != nil {
		return nil, err
	}

	fallback := fmt.Sprintf("Media saved to %s (%s)", resp.FilePath, resp.MediaType)
	return mcp.NewToolResultStructured(resp, fallback), nil
}

func toBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("unable to parse boolean value %q", v)
		}
		return parsed, nil
	case float64:
		return v != 0, nil
	case int:
		return v != 0, nil
	default:
		return false, fmt.Errorf("unsupported boolean value type %T", value)
	}
}
