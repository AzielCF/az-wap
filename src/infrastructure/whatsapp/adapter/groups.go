package adapter

import (
	"context"

	"github.com/AzielCF/az-wap/workspace/domain/common"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func (wa *WhatsAppAdapter) CreateGroup(ctx context.Context, name string, participants []string) (string, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	cli := wa.client
	pJIDs := make([]types.JID, 0, len(participants))
	for _, p := range participants {
		pj, err := wa.parseJID(p)
		if err == nil {
			pJIDs = append(pJIDs, pj)
		}
	}
	resp, err := cli.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: pJIDs,
	})
	if err != nil {
		return "", err
	}
	return resp.JID.String(), nil
}

func (wa *WhatsAppAdapter) JoinGroupWithLink(ctx context.Context, link string) (string, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	cli := wa.client
	jid, err := cli.JoinGroupWithLink(ctx, link)
	if err != nil {
		return "", err
	}
	return jid.String(), nil
}

func (wa *WhatsAppAdapter) LeaveGroup(ctx context.Context, groupID string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return err
	}
	cli := wa.client
	return cli.LeaveGroup(ctx, jid)
}

func (wa *WhatsAppAdapter) GetGroupInfo(ctx context.Context, groupID string) (common.GroupInfo, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return common.GroupInfo{}, err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return common.GroupInfo{}, err
	}
	cli := wa.client
	info, err := cli.GetGroupInfo(ctx, jid)
	if err != nil {
		return common.GroupInfo{}, err
	}
	result := common.GroupInfo{
		JID:        info.JID.String(),
		Name:       info.GroupName.Name,
		OwnerJID:   info.OwnerJID.String(),
		CreateTime: info.GroupCreated,
		IsLocked:   info.IsLocked,
		IsAnnounce: info.IsAnnounce,
	}
	for _, p := range info.Participants {
		result.Participants = append(result.Participants, common.GroupParticipant{
			JID:          p.JID.String(),
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
			DisplayName:  p.DisplayName,
		})
	}
	return result, nil
}

func (wa *WhatsAppAdapter) UpdateGroupParticipants(ctx context.Context, groupID string, participants []string, action common.ParticipantAction) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	gJID, err := wa.parseJID(groupID)
	if err != nil {
		return err
	}

	pJIDs := make([]types.JID, 0, len(participants))
	for _, p := range participants {
		if j, err := wa.parseJID(p); err == nil {
			pJIDs = append(pJIDs, j)
		}
	}

	waAction := whatsmeow.ParticipantChangeAdd
	switch action {
	case common.ParticipantActionRemove:
		waAction = whatsmeow.ParticipantChangeRemove
	case common.ParticipantActionPromote:
		waAction = whatsmeow.ParticipantChangePromote
	case common.ParticipantActionDemote:
		waAction = whatsmeow.ParticipantChangeDemote
	}

	cli := wa.client
	_, err = cli.UpdateGroupParticipants(ctx, gJID, pJIDs, waAction)
	return err
}

func (wa *WhatsAppAdapter) GetGroupInviteLink(ctx context.Context, groupID string, reset bool) (string, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return "", err
	}
	cli := wa.client
	return cli.GetGroupInviteLink(ctx, jid, reset)
}

func (wa *WhatsAppAdapter) GetJoinedGroups(ctx context.Context) ([]common.GroupInfo, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return nil, err
	}
	cli := wa.client
	groups, err := cli.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]common.GroupInfo, 0, len(groups))
	for _, g := range groups {
		result = append(result, common.GroupInfo{
			JID:        g.JID.String(),
			Name:       g.Name,
			OwnerJID:   g.OwnerJID.String(),
			CreateTime: g.GroupCreated,
		})
	}
	return result, nil
}

func (wa *WhatsAppAdapter) GetGroupInfoFromLink(ctx context.Context, link string) (common.GroupInfo, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return common.GroupInfo{}, err
	}
	cli := wa.client
	info, err := cli.GetGroupInfoFromLink(ctx, link)
	if err != nil {
		return common.GroupInfo{}, err
	}
	return common.GroupInfo{
		JID:        info.JID.String(),
		Name:       info.Name,
		CreateTime: info.GroupCreated,
	}, nil
}

func (wa *WhatsAppAdapter) GetGroupRequestParticipants(ctx context.Context, groupID string) ([]common.GroupRequestParticipant, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return nil, err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return nil, err
	}
	cli := wa.client
	reqs, err := cli.GetGroupRequestParticipants(ctx, jid)
	if err != nil {
		return nil, err
	}
	result := make([]common.GroupRequestParticipant, 0, len(reqs))
	for _, r := range reqs {
		result = append(result, common.GroupRequestParticipant{
			JID:         r.JID.String(),
			RequestedAt: r.RequestedAt,
		})
	}
	return result, nil
}

func (wa *WhatsAppAdapter) UpdateGroupRequestParticipants(ctx context.Context, groupID string, participants []string, action common.ParticipantAction) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	_, err := wa.parseJID(groupID)
	if err != nil {
		return err
	}
	pJIDs := make([]types.JID, 0, len(participants))
	for _, p := range participants {
		// user JID
		if j, err := wa.parseJID(p); err == nil {
			pJIDs = append(pJIDs, j)
		}
	}

	// Assume Approve or Reject
	for range pJIDs {
		// Placeholder for future implementation
	}
	return nil
}

func (wa *WhatsAppAdapter) SetGroupName(ctx context.Context, groupID string, name string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return err
	}
	cli := wa.client
	return cli.SetGroupName(ctx, jid, name)
}

func (wa *WhatsAppAdapter) SetGroupLocked(ctx context.Context, groupID string, locked bool) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return err
	}
	cli := wa.client
	return cli.SetGroupLocked(ctx, jid, locked)
}

func (wa *WhatsAppAdapter) SetGroupAnnounce(ctx context.Context, groupID string, announce bool) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return err
	}
	cli := wa.client
	return cli.SetGroupAnnounce(ctx, jid, announce)
}

func (wa *WhatsAppAdapter) SetGroupTopic(ctx context.Context, groupID string, topic string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return err
	}
	cli := wa.client
	return cli.SetGroupTopic(ctx, jid, "", "", topic)
}

func (wa *WhatsAppAdapter) SetGroupPhoto(ctx context.Context, groupID string, photo []byte) (string, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	jid, err := wa.parseJID(groupID)
	if err != nil {
		return "", err
	}
	cli := wa.client
	return cli.SetGroupPhoto(ctx, jid, photo)
}
