package adapter

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"

	pkgUtils "github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/workspace/domain/common"
)

func (wa *WhatsAppAdapter) SetProfileName(ctx context.Context, name string) error {
	if wa.client == nil {
		return fmt.Errorf("no client")
	}
	return wa.client.SendAppState(ctx, appstate.BuildSettingPushName(name))
}

func (wa *WhatsAppAdapter) SetProfileStatus(ctx context.Context, status string) error {
	if wa.client == nil {
		return fmt.Errorf("no client")
	}
	return wa.client.SetStatusMessage(ctx, status)
}

func (wa *WhatsAppAdapter) SetProfilePhoto(ctx context.Context, photo []byte) (string, error) {
	if wa.client == nil {
		return "", fmt.Errorf("no client")
	}
	// Personal profile photo in whatsmeow is set using SetGroupPhoto with empty JID
	resp, err := wa.client.SetGroupPhoto(ctx, types.JID{}, photo)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (wa *WhatsAppAdapter) GetContact(ctx context.Context, jid string) (common.ContactInfo, error) {
	if wa.client == nil {
		return common.ContactInfo{}, fmt.Errorf("no client")
	}

	// Handle potential combined identity (pn|lid) or just LID/JID
	targetJID := jid
	if strings.Contains(jid, "|") {
		parts := strings.Split(jid, "|")
		// Prefer LID if available in the combined string
		for _, p := range parts {
			if strings.HasSuffix(p, "@lid") {
				targetJID = p
				break
			}
		}
		if targetJID == jid {
			targetJID = parts[0]
		}
	}

	parsedJID, err := wa.parseJID(targetJID)
	if err != nil {
		return common.ContactInfo{}, err
	}

	// Try store first
	contact, err := wa.client.Store.Contacts.GetContact(ctx, parsedJID)
	if err == nil && contact.Found {
		return common.ContactInfo{
			JID:  parsedJID.String(),
			Name: contact.FullName,
		}, nil
	}

	// Fallback to network query
	info, err := wa.client.GetUserInfo(ctx, []types.JID{parsedJID})
	if err != nil || len(info) == 0 {
		return common.ContactInfo{JID: targetJID}, nil
	}

	user := info[parsedJID]
	vName := ""
	if user.VerifiedName != nil {
		vName = fmt.Sprintf("%v", user.VerifiedName)
	}

	return common.ContactInfo{
		JID:    parsedJID.String(),
		Name:   vName,
		Status: user.Status,
	}, nil
}

func (wa *WhatsAppAdapter) GetPrivacySettings(ctx context.Context) (common.PrivacySettings, error) {
	if wa.client == nil {
		return common.PrivacySettings{}, fmt.Errorf("no client")
	}
	resp, err := wa.client.TryFetchPrivacySettings(ctx, true)
	if err != nil {
		return common.PrivacySettings{}, err
	}
	return common.PrivacySettings{
		GroupAdd:     string(resp.GroupAdd),
		Status:       string(resp.Status),
		ReadReceipts: string(resp.ReadReceipts),
		Profile:      string(resp.Profile),
	}, nil
}

func (wa *WhatsAppAdapter) GetUserInfo(ctx context.Context, jids []string) ([]common.ContactInfo, error) {
	if wa.client == nil {
		return nil, fmt.Errorf("no client")
	}
	parsedJIDs := make([]types.JID, 0, len(jids))
	for _, j := range jids {
		pj, err := wa.parseJID(j)
		if err == nil {
			parsedJIDs = append(parsedJIDs, pj)
		}
	}
	resp, err := wa.client.GetUserInfo(ctx, parsedJIDs)
	if err != nil {
		return nil, err
	}
	result := make([]common.ContactInfo, 0, len(resp))
	for jid, info := range resp {
		result = append(result, common.ContactInfo{
			JID:    jid.String(),
			Name:   info.Status, // Minimal mapping based on available info
			Status: info.Status,
		})
	}
	return result, nil
}

func (wa *WhatsAppAdapter) GetProfilePictureInfo(ctx context.Context, jid string, preview bool) (string, error) {
	if wa.client == nil {
		return "", fmt.Errorf("no client")
	}
	parsedJID, err := wa.parseJID(jid)
	if err != nil {
		return "", err
	}
	pic, err := wa.client.GetProfilePictureInfo(ctx, parsedJID, &whatsmeow.GetProfilePictureParams{
		Preview: preview,
	})
	if err != nil {
		return "", err
	}
	if pic == nil {
		return "", fmt.Errorf("no profile picture")
	}
	return pic.URL, nil
}

func (wa *WhatsAppAdapter) GetBusinessProfile(ctx context.Context, jid string) (common.BusinessProfile, error) {
	if wa.client == nil {
		return common.BusinessProfile{}, fmt.Errorf("no client")
	}
	parsedJID, err := wa.parseJID(jid)
	if err != nil {
		return common.BusinessProfile{}, err
	}
	profile, err := wa.client.GetBusinessProfile(ctx, parsedJID)
	if err != nil {
		return common.BusinessProfile{}, err
	}

	res := common.BusinessProfile{
		JID:                   jid,
		Email:                 profile.Email,
		Address:               profile.Address,
		BusinessHoursTimeZone: profile.BusinessHoursTimeZone,
	}

	for _, cat := range profile.Categories {
		res.Categories = append(res.Categories, common.BusinessCategory{ID: cat.ID, Name: cat.Name})
	}

	for _, hr := range profile.BusinessHours {
		res.BusinessHours = append(res.BusinessHours, common.BusinessHourDay{
			DayOfWeek: hr.DayOfWeek,
			Mode:      hr.Mode,
			OpenTime:  fmt.Sprintf("%v", hr.OpenTime),
			CloseTime: fmt.Sprintf("%v", hr.CloseTime),
		})
	}

	return res, nil
}

func (wa *WhatsAppAdapter) GetAllContacts(ctx context.Context) ([]common.ContactInfo, error) {
	if wa.client == nil {
		return nil, fmt.Errorf("no client")
	}
	contacts, err := wa.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]common.ContactInfo, 0, len(contacts))
	for jid, contact := range contacts {
		result = append(result, common.ContactInfo{
			JID:  jid.String(),
			Name: contact.FullName,
		})
	}
	return result, nil
}

func (wa *WhatsAppAdapter) IsOnWhatsApp(ctx context.Context, phone string) (bool, error) {
	if wa.client == nil {
		return false, fmt.Errorf("no client")
	}
	// Basic sanitization
	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, " ", "")

	// whatsmeow IsOnWhatsApp needs a context and a slice
	resp, err := wa.client.IsOnWhatsApp(ctx, []string{phone})
	if err != nil {
		return false, err
	}
	for _, res := range resp {
		if res.IsIn {
			return true, nil
		}
	}
	return false, nil
}

func (wa *WhatsAppAdapter) ResolveIdentity(ctx context.Context, identifier string) (string, error) {
	if wa.client == nil {
		return identifier, fmt.Errorf("no client available")
	}

	var targetJID types.JID
	var err error

	// 1. Determine base JID
	if strings.Contains(identifier, "@") {
		targetJID, err = types.ParseJID(identifier)
		if err != nil {
			return identifier, fmt.Errorf("invalid identifier format")
		}
	} else {
		// Clean and normalize number
		id := pkgUtils.ExtractPhoneNumber(identifier)
		if id == "" {
			return identifier, fmt.Errorf("could not extract a valid phone number")
		}

		// Try to verify on WhatsApp
		resp, err := wa.client.IsOnWhatsApp(ctx, []string{id})
		if err != nil || len(resp) == 0 || !resp[0].IsIn {
			// Second attempt: maybe it needs a prefix or is already a partial jid-like
			if !strings.HasSuffix(id, "@s.whatsapp.net") {
				testJID := types.NewJID(id, types.DefaultUserServer)
				info, err := wa.client.GetUserInfo(ctx, []types.JID{testJID})
				if err == nil && len(info) > 0 {
					targetJID = testJID
				} else {
					return identifier, fmt.Errorf("identity not found on WhatsApp")
				}
			} else {
				return identifier, fmt.Errorf("identity not found on WhatsApp")
			}
		} else {
			targetJID = resp[0].JID
		}
	}

	// 2. Resolve LID via GetUserInfo - USER REQUIRES ONLY LID, NEVER JID
	info, err := wa.client.GetUserInfo(ctx, []types.JID{targetJID})
	if err == nil {
		if u, ok := info[targetJID]; ok {
			lid := u.LID
			if !lid.IsEmpty() {
				return lid.String(), nil
			}
		}
	}

	// NO LID found - return error, never fallback to JID
	return "", fmt.Errorf("could not resolve LID for this identity")
}
