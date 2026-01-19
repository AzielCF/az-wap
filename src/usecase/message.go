package usecase

import (
	"context"
	"fmt"

	domainMessage "github.com/AzielCF/az-wap/domains/message"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
)

type serviceMessage struct {
	workspaceMgr *workspace.Manager
}

func NewMessageService(workspaceMgr *workspace.Manager) domainMessage.IMessageUsecase {
	return &serviceMessage{
		workspaceMgr: workspaceMgr,
	}
}

func (service serviceMessage) getAdapterForToken(ctx context.Context, token string) (wsChannelDomain.ChannelAdapter, error) {
	if token == "" {
		return nil, fmt.Errorf("token missing")
	}

	adapter, ok := service.workspaceMgr.GetAdapter(token)
	if !ok {
		return nil, fmt.Errorf("channel adapter %s not found or not active", token)
	}

	return adapter, nil
}

func (service serviceMessage) MarkAsRead(ctx context.Context, request domainMessage.MarkAsReadRequest) (response domainMessage.GenericResponse, err error) {
	if err = validations.ValidateMarkAsRead(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	if err = adapter.MarkRead(ctx, request.Phone, []string{request.MessageID}); err != nil {
		return response, err
	}

	response.MessageID = request.MessageID
	response.Status = fmt.Sprintf("Mark as read success %s", request.MessageID)
	return response, nil
}

func (service serviceMessage) ReactMessage(ctx context.Context, request domainMessage.ReactionRequest) (response domainMessage.GenericResponse, err error) {
	if err = validations.ValidateReactMessage(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	msgID, err := adapter.ReactMessage(ctx, request.Phone, request.MessageID, request.Emoji)
	if err != nil {
		return response, err
	}

	response.MessageID = msgID
	response.Status = fmt.Sprintf("Reaction sent to %s", request.Phone)
	return response, nil
}

func (service serviceMessage) RevokeMessage(ctx context.Context, request domainMessage.RevokeRequest) (response domainMessage.GenericResponse, err error) {
	if err = validations.ValidateRevokeMessage(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	msgID, err := adapter.RevokeMessage(ctx, request.Phone, request.MessageID)
	if err != nil {
		return response, err
	}

	response.MessageID = msgID
	response.Status = fmt.Sprintf("Revoke success %s", request.Phone)
	return response, nil
}

func (service serviceMessage) DeleteMessage(ctx context.Context, request domainMessage.DeleteRequest) (err error) {
	if err = validations.ValidateDeleteMessage(ctx, request); err != nil {
		return err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.DeleteMessageForMe(ctx, request.Phone, request.MessageID)
}

func (service serviceMessage) UpdateMessage(ctx context.Context, request domainMessage.UpdateMessageRequest) (response domainMessage.GenericResponse, err error) {
	if err = validations.ValidateUpdateMessage(ctx, request); err != nil {
		return response, err
	}

	// NOTE: Edit/Update message not directly in ChannelAdapter yet for text.
	// The adapter usually handles SendMessage. WhatsApp Edit is a specific call.
	// We might need to add it to ChannelAdapter or use SendMessage with edit flag?
	// For now returns error or unimplemented.
	return response, fmt.Errorf("UpdateMessage not implemented in adapter yet")
}

func (service serviceMessage) StarMessage(ctx context.Context, request domainMessage.StarRequest) (err error) {
	if err = validations.ValidateStarMessage(ctx, request); err != nil {
		return err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.StarMessage(ctx, request.Phone, request.MessageID, request.IsStarred)
}

func (service serviceMessage) DownloadMedia(ctx context.Context, request domainMessage.DownloadMediaRequest) (response domainMessage.DownloadMediaResponse, err error) {
	if err = validations.ValidateDownloadMedia(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	path, err := adapter.DownloadMedia(ctx, request.MessageID, request.Phone)
	if err != nil {
		return response, err
	}

	response.MessageID = request.MessageID
	response.Status = "Media downloaded successfully"
	response.FilePath = path
	return response, nil
}
