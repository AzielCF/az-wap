package usecase

import (
	"context"
	"fmt"

	domainChat "github.com/AzielCF/az-wap/domains/chat"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/sirupsen/logrus"
)

type serviceChat struct {
	workspaceMgr *workspace.Manager
}

func NewChatService(workspaceMgr *workspace.Manager) domainChat.IChatUsecase {
	return &serviceChat{
		workspaceMgr: workspaceMgr,
	}
}

func (service serviceChat) ListChats(ctx context.Context, request domainChat.ListChatsRequest) (domainChat.ListChatsResponse, error) {
	// Persistent chat history is discontinued. Returning empty list.
	return domainChat.ListChatsResponse{
		Data: []domainChat.ChatInfo{},
		Pagination: domainChat.PaginationResponse{
			Limit:  request.Limit,
			Offset: request.Offset,
			Total:  0,
		},
	}, nil
}

func (service serviceChat) GetChatMessages(ctx context.Context, request domainChat.GetChatMessagesRequest) (domainChat.GetChatMessagesResponse, error) {
	// Persistent chat history is discontinued. Returning empty list.
	return domainChat.GetChatMessagesResponse{
		Data: []domainChat.MessageInfo{},
		Pagination: domainChat.PaginationResponse{
			Limit:  request.Limit,
			Offset: request.Offset,
			Total:  0,
		},
	}, nil
}

func (service serviceChat) PinChat(ctx context.Context, request domainChat.PinChatRequest) (domainChat.PinChatResponse, error) {
	var response domainChat.PinChatResponse
	if err := validations.ValidatePinChat(ctx, &request); err != nil {
		return response, err
	}

	adapter, ok := service.workspaceMgr.GetAdapter(request.Token)
	if !ok {
		return response, fmt.Errorf("channel adapter %s not found or not active", request.Token)
	}

	if err := adapter.PinChat(ctx, request.ChatJID, request.Pinned); err != nil {
		logrus.WithError(err).Error("Failed to pin chat via adapter")
		return response, err
	}

	response.Status = "success"
	response.ChatJID = request.ChatJID
	response.Pinned = request.Pinned
	if request.Pinned {
		response.Message = "Chat pinned successfully"
	} else {
		response.Message = "Chat unpinned successfully"
	}

	return response, nil
}
