package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"

	pkgUtils "github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/sirupsen/logrus"
)

func (wa *WhatsAppAdapter) SetProfileName(ctx context.Context, name string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	cli := wa.client
	return cli.SendAppState(ctx, appstate.BuildSettingPushName(name))
}

func (wa *WhatsAppAdapter) SetProfileStatus(ctx context.Context, status string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	cli := wa.client
	return cli.SetStatusMessage(ctx, status)
}

func (wa *WhatsAppAdapter) SetProfilePhoto(ctx context.Context, photo []byte) (string, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	cli := wa.client
	if !cli.IsConnected() {
		return "", fmt.Errorf("client not connected")
	}

	// For personal profile photo, the protocol destination must be Empty JID.
	// Even on LID-based connections, using 'Empty JID' tells WhatsApp: "This is for the authenticated user".
	// Using a specific LID JID here often causes it to be treated as a GROUP operation, which fails with timeout.
	targetID := types.JID{}

	logrus.Infof("[WHATSAPP_ADAPTER] Setting personal profile photo (using Empty JID standard)")

	resp, err := cli.SetGroupPhoto(ctx, targetID, photo)
	if err != nil {
		logrus.WithError(err).Errorf("[WHATSAPP_ADAPTER] SetGroupPhoto failed for self-profile update")
		return "", err
	}
	return resp, nil
}

func (wa *WhatsAppAdapter) GetContact(ctx context.Context, jid string) (common.ContactInfo, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return common.ContactInfo{}, err
	}
	cli := wa.client
	// ... rest of the function using cli ...
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
	contact, err := cli.Store.Contacts.GetContact(ctx, parsedJID)
	if err == nil && contact.Found {
		return common.ContactInfo{
			JID:  parsedJID.String(),
			Name: contact.FullName,
		}, nil
	}

	// Fallback to network query
	info, err := cli.GetUserInfo(ctx, []types.JID{parsedJID})
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
	if err := wa.ensureConnected(ctx); err != nil {
		return common.PrivacySettings{}, err
	}
	cli := wa.client
	resp, err := cli.TryFetchPrivacySettings(ctx, true)
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
	if err := wa.ensureConnected(ctx); err != nil {
		return nil, err
	}
	cli := wa.client
	// ... (parsedJIDs loop here)
	parsedJIDs := make([]types.JID, 0, len(jids))
	for _, j := range jids {
		pj, err := wa.parseJID(j)
		if err == nil {
			parsedJIDs = append(parsedJIDs, pj)
		}
	}
	resp, err := cli.GetUserInfo(ctx, parsedJIDs)
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
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	cli := wa.client
	parsedJID, err := wa.parseJID(jid)
	if err != nil {
		return "", err
	}
	pic, err := cli.GetProfilePictureInfo(ctx, parsedJID, &whatsmeow.GetProfilePictureParams{
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
	if err := wa.ensureConnected(ctx); err != nil {
		return common.BusinessProfile{}, err
	}
	cli := wa.client
	parsedJID, err := wa.parseJID(jid)
	if err != nil {
		return common.BusinessProfile{}, err
	}
	profile, err := cli.GetBusinessProfile(ctx, parsedJID)
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
	if err := wa.ensureConnected(ctx); err != nil {
		return nil, err
	}
	cli := wa.client
	contacts, err := cli.Store.Contacts.GetAllContacts(ctx)
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
	if err := wa.ensureConnected(ctx); err != nil {
		return false, err
	}
	cli := wa.client
	// Basic sanitization
	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, " ", "")

	// whatsmeow IsOnWhatsApp needs a context and a slice
	resp, err := cli.IsOnWhatsApp(ctx, []string{phone})
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
	if err := wa.ensureConnected(ctx); err != nil {
		return identifier, err
	}
	cli := wa.client

	// 0. Ensure client is connected and logged in before attempting resolution
	if !cli.IsLoggedIn() {
		return identifier, fmt.Errorf("client not logged in")
	}

	logrus.WithFields(logrus.Fields{
		"identifier": identifier,
		"channel":    wa.channelID,
	}).Info("[WHATSAPP] Resolving identity...")

	var targetJID types.JID
	var err error

	// Create a sub-context with timeout to avoid getting stuck
	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

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

		// LOGIC: We will try 3 paths to ensure we find the user
		// Path A: IsOnWhatsApp with raw digits
		// Path B: IsOnWhatsApp with @s.whatsapp.net suffix
		// Path C: GetUserInfo directly

		found := false

		// Attempt Path A
		resp, err := cli.IsOnWhatsApp(queryCtx, []string{id})
		if err == nil && len(resp) > 0 && resp[0].IsIn {
			targetJID = resp[0].JID
			found = true
			logrus.Debugf("[WHATSAPP] Found %s via Path A (Raw IsOnWA)", id)
		} else {
			// Attempt Path B
			jidStr := id + "@s.whatsapp.net"
			resp, err := cli.IsOnWhatsApp(queryCtx, []string{jidStr})
			if err == nil && len(resp) > 0 && resp[0].IsIn {
				targetJID = resp[0].JID
				found = true
				logrus.Debugf("[WHATSAPP] Found %s via Path B (JID IsOnWA)", id)
			}
		}

		if !found {
			// Attempt Path C: GetUserInfo
			testJID := types.NewJID(id, types.DefaultUserServer)
			info, err := cli.GetUserInfo(queryCtx, []types.JID{testJID})
			if err == nil && len(info) > 0 {
				targetJID = testJID
				found = true
				logrus.Debugf("[WHATSAPP] Found %s via Path C (GetUserInfo)", id)
			}
		}

		if !found {
			logrus.Warnf("[WHATSAPP] %s NOT FOUND on WhatsApp after trying all paths.", id)
			return identifier, fmt.Errorf("identity not found on WhatsApp")
		}
	}

	// 2. Resolve LID via GetUserInfo - USER REQUIRES ONLY LID, NEVER JID
	// Use a fresh timeout context for LID resolution too
	lidCtx, lidCancel := context.WithTimeout(ctx, 10*time.Second)
	defer lidCancel()

	info, err := cli.GetUserInfo(lidCtx, []types.JID{targetJID})
	if err == nil {
		if u, ok := info[targetJID]; ok {
			lid := u.LID
			if !lid.IsEmpty() {
				logrus.WithFields(logrus.Fields{
					"target": targetJID.String(),
					"lid":    lid.String(),
				}).Info("[WHATSAPP] LID Resolved successfully")
				return lid.String(), nil
			}
		}
	}

	logrus.Warnf("[WHATSAPP] Could not resolve LID for %s (JID found: %s, Error: %v)", identifier, targetJID, err)
	// NO LID found - return error, never fallback to JID
	return "", fmt.Errorf("could not resolve LID for this identity")
}
