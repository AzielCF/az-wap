package usecase

import (
	"context"
	"fmt"

	domainGroup "github.com/AzielCF/az-wap/domains/group"
	pkgUtils "github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	wsCommonDomain "github.com/AzielCF/az-wap/workspace/domain/common"
)

type serviceGroup struct {
	workspaceMgr *workspace.Manager
}

func NewGroupService(workspaceMgr *workspace.Manager) domainGroup.IGroupUsecase {
	return &serviceGroup{
		workspaceMgr: workspaceMgr,
	}
}

func (service serviceGroup) getAdapterForToken(ctx context.Context, token string) (wsChannelDomain.ChannelAdapter, error) {
	if token == "" || service.workspaceMgr == nil {
		return nil, fmt.Errorf("workspace manager or token missing")
	}

	adapter, ok := service.workspaceMgr.GetAdapter(token)
	if !ok {
		return nil, fmt.Errorf("channel adapter %s not found or not active", token)
	}

	return adapter, nil
}

func (service serviceGroup) JoinGroupWithLink(ctx context.Context, request domainGroup.JoinGroupWithLinkRequest) (groupID string, err error) {
	if err = validations.ValidateJoinGroupWithLink(ctx, request); err != nil {
		return groupID, err
	}
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return groupID, err
	}

	return adapter.JoinGroupWithLink(ctx, request.Link)
}

func (service serviceGroup) LeaveGroup(ctx context.Context, request domainGroup.LeaveGroupRequest) (err error) {
	if err = validations.ValidateLeaveGroup(ctx, request); err != nil {
		return err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.LeaveGroup(ctx, request.GroupID)
}

func (service serviceGroup) CreateGroup(ctx context.Context, request domainGroup.CreateGroupRequest) (groupID string, err error) {
	if err = validations.ValidateCreateGroup(ctx, request); err != nil {
		return groupID, err
	}
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return groupID, err
	}

	return adapter.CreateGroup(ctx, request.Title, request.Participants)
}

func (service serviceGroup) GetGroupInfoFromLink(ctx context.Context, request domainGroup.GetGroupInfoFromLinkRequest) (response domainGroup.GetGroupInfoFromLinkResponse, err error) {
	if err = validations.ValidateGetGroupInfoFromLink(ctx, request); err != nil {
		return response, err
	}
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	groupInfo, err := adapter.GetGroupInfoFromLink(ctx, request.Link)
	if err != nil {
		return response, err
	}

	response = domainGroup.GetGroupInfoFromLinkResponse{
		GroupID:     groupInfo.JID,
		Name:        groupInfo.Name,
		CreatedAt:   groupInfo.CreateTime,
		Description: groupInfo.Name,
	}

	return response, nil
}

func (service serviceGroup) ManageParticipant(ctx context.Context, request domainGroup.ParticipantRequest) (result []domainGroup.ParticipantStatus, err error) {
	if err = validations.ValidateParticipant(ctx, request); err != nil {
		return result, err
	}
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return result, err
	}

	err = adapter.UpdateGroupParticipants(ctx, request.GroupID, request.Participants, wsCommonDomain.ParticipantAction(request.Action))
	if err != nil {
		return result, err
	}

	// Result mapping is detailed in old service, but we'll simplified for now
	return result, nil
}

func (service serviceGroup) GetGroupParticipants(ctx context.Context, request domainGroup.GetGroupParticipantsRequest) (response domainGroup.GetGroupParticipantsResponse, err error) {
	if err = validations.ValidateGetGroupParticipants(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	groupInfo, err := adapter.GetGroupInfo(ctx, request.GroupID)
	if err != nil {
		return response, err
	}

	response.GroupID = groupInfo.JID
	response.Name = groupInfo.Name
	for _, p := range groupInfo.Participants {
		response.Participants = append(response.Participants, domainGroup.GroupParticipant{
			JID:          p.JID,
			DisplayName:  p.DisplayName,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		})
	}

	return response, nil
}

func (service serviceGroup) GetGroupRequestParticipants(ctx context.Context, request domainGroup.GetGroupRequestParticipantsRequest) (response []domainGroup.GetGroupRequestParticipantsResponse, err error) {
	if err = validations.ValidateGetGroupRequestParticipants(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	participants, err := adapter.GetGroupRequestParticipants(ctx, request.GroupID)
	if err != nil {
		return response, err
	}

	for _, p := range participants {
		response = append(response, domainGroup.GetGroupRequestParticipantsResponse{
			JID:         p.JID,
			RequestedAt: p.RequestedAt,
		})
	}

	return response, nil
}

func (service serviceGroup) ManageGroupRequestParticipants(ctx context.Context, request domainGroup.GroupRequestParticipantsRequest) (result []domainGroup.ParticipantStatus, err error) {
	if err = validations.ValidateManageGroupRequestParticipants(ctx, request); err != nil {
		return result, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return result, err
	}

	err = adapter.UpdateGroupRequestParticipants(ctx, request.GroupID, request.Participants, wsCommonDomain.ParticipantAction(request.Action))
	if err != nil {
		return result, err
	}

	return result, nil
}

func (service serviceGroup) SetGroupPhoto(ctx context.Context, request domainGroup.SetGroupPhotoRequest) (pictureID string, err error) {
	if err = validations.ValidateSetGroupPhoto(ctx, request); err != nil {
		return pictureID, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return pictureID, err
	}

	var photoBytes []byte
	if request.Photo != nil {
		processedImageBuffer, err := pkgUtils.ProcessGroupPhoto(request.Photo)
		if err != nil {
			return pictureID, err
		}
		photoBytes = processedImageBuffer.Bytes()
	}

	return adapter.SetGroupPhoto(ctx, request.GroupID, photoBytes)
}

func (service serviceGroup) SetGroupName(ctx context.Context, request domainGroup.SetGroupNameRequest) (err error) {
	if err = validations.ValidateSetGroupName(ctx, request); err != nil {
		return err
	}
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.SetGroupName(ctx, request.GroupID, request.Name)
}

func (service serviceGroup) SetGroupLocked(ctx context.Context, request domainGroup.SetGroupLockedRequest) (err error) {
	if err = validations.ValidateSetGroupLocked(ctx, request); err != nil {
		return err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.SetGroupLocked(ctx, request.GroupID, request.Locked)
}

func (service serviceGroup) SetGroupAnnounce(ctx context.Context, request domainGroup.SetGroupAnnounceRequest) (err error) {
	if err = validations.ValidateSetGroupAnnounce(ctx, request); err != nil {
		return err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.SetGroupAnnounce(ctx, request.GroupID, request.Announce)
}

func (service serviceGroup) SetGroupTopic(ctx context.Context, request domainGroup.SetGroupTopicRequest) (err error) {
	if err = validations.ValidateSetGroupTopic(ctx, request); err != nil {
		return err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.SetGroupTopic(ctx, request.GroupID, request.Topic)
}

// GroupInfo retrieves detailed information about a WhatsApp group
func (service serviceGroup) GroupInfo(ctx context.Context, request domainGroup.GroupInfoRequest) (response domainGroup.GroupInfoResponse, err error) {
	// Validate the incoming request
	if err = validations.ValidateGroupInfo(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	// Fetch group information from WhatsApp
	groupInfo, err := adapter.GetGroupInfo(ctx, request.GroupID)
	if err != nil {
		return response, err
	}

	// Map the response
	response.Data = groupInfo

	return response, nil
}

func (service serviceGroup) GetGroupInviteLink(ctx context.Context, request domainGroup.GetGroupInviteLinkRequest) (response domainGroup.GetGroupInviteLinkResponse, err error) {
	if err = validations.ValidateGetGroupInviteLink(ctx, request); err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	inviteLink, err := adapter.GetGroupInviteLink(ctx, request.GroupID, request.Reset)
	if err != nil {
		return response, err
	}

	response = domainGroup.GetGroupInviteLinkResponse{
		InviteLink: inviteLink,
		GroupID:    request.GroupID,
	}

	return response, nil
}
