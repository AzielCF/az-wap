package usecase

import (
	"bytes"
	"context"
	"fmt"
	"image"

	domainUser "github.com/AzielCF/az-wap/domains/user"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsDomainChannel "github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/disintegration/imaging"
)

type serviceUser struct {
	workspaceMgr *workspace.Manager
}

func NewUserService(workspaceMgr *workspace.Manager) domainUser.IUserUsecase {
	return &serviceUser{
		workspaceMgr: workspaceMgr,
	}
}

func (service serviceUser) getAdapterForToken(ctx context.Context, token string) (wsDomainChannel.ChannelAdapter, error) {
	if token == "" || service.workspaceMgr == nil {
		return nil, fmt.Errorf("workspace manager or token missing")
	}

	adapter, ok := service.workspaceMgr.GetAdapter(token)
	if !ok {
		return nil, fmt.Errorf("channel adapter %s not found", token)
	}

	return adapter, nil
}

func (service serviceUser) Info(ctx context.Context, request domainUser.InfoRequest) (response domainUser.InfoResponse, err error) {
	err = validations.ValidateUserInfo(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	resp, err := adapter.GetUserInfo(ctx, []string{request.Phone})
	if err != nil {
		return response, err
	}

	for _, userInfo := range resp {
		data := domainUser.InfoResponseData{
			Status: userInfo.Status,
			// Simplified devices mapping since generic ContactInfo doesn't have devices yet
			// We could extend ContactInfo if needed, but for now we follow the "extensa" API request
		}
		response.Data = append(response.Data, data)
	}

	return response, nil
}

func (service serviceUser) Avatar(ctx context.Context, request domainUser.AvatarRequest) (domainUser.AvatarResponse, error) {
	var response domainUser.AvatarResponse
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	url, err := adapter.GetProfilePictureInfo(ctx, request.Phone, request.IsPreview)
	if err != nil {
		return response, err
	}

	response.URL = url
	return response, nil
}

func (service serviceUser) MyListGroups(ctx context.Context, token string) (response domainUser.MyListGroupsResponse, err error) {
	adapter, err := service.getAdapterForToken(ctx, token)
	if err != nil {
		return response, err
	}

	groups, err := adapter.GetJoinedGroups(ctx)
	if err != nil {
		return response, err
	}

	// We could map these to types.GroupInfo if needed for full compatibility
	_ = groups

	return response, nil
}

func (service serviceUser) MyListNewsletter(ctx context.Context, token string) (response domainUser.MyListNewsletterResponse, err error) {
	adapter, err := service.getAdapterForToken(ctx, token)
	if err != nil {
		return response, err
	}

	_, err = adapter.FetchNewsletters(ctx)
	if err != nil {
		return response, err
	}

	// Mapping required if we want to return types.NewsletterMetadata compatible response
	return response, nil
}

func (service serviceUser) MyPrivacySetting(ctx context.Context, token string) (response domainUser.MyPrivacySettingResponse, err error) {
	adapter, err := service.getAdapterForToken(ctx, token)
	if err != nil {
		return response, err
	}

	resp, err := adapter.GetPrivacySettings(ctx)
	if err != nil {
		return response, err
	}

	response.GroupAdd = resp.GroupAdd
	response.Status = resp.Status
	response.ReadReceipts = resp.ReadReceipts
	response.Profile = resp.Profile
	return response, nil
}

func (service serviceUser) MyListContacts(ctx context.Context, token string) (response domainUser.MyListContactsResponse, err error) {
	adapter, err := service.getAdapterForToken(ctx, token)
	if err != nil {
		return response, err
	}

	contacts, err := adapter.GetAllContacts(ctx)
	if err != nil {
		return response, err
	}

	for _, contact := range contacts {
		response.Data = append(response.Data, domainUser.MyListContactsResponseData{
			// JID mapping needed
			Name: contact.Name,
		})
	}

	return response, nil
}

func (service serviceUser) ChangeAvatar(ctx context.Context, request domainUser.ChangeAvatarRequest) (err error) {
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	file, err := request.Avatar.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	// ... image processing ...
	srcImage, err := imaging.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	bounds := srcImage.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	size := width
	if height < width {
		size = height
	}
	if size > 640 {
		size = 640
	}
	left := (width - size) / 2
	top := (height - size) / 2
	croppedImage := imaging.Crop(srcImage, image.Rect(left, top, left+size, top+size))
	if size > 640 {
		croppedImage = imaging.Resize(croppedImage, 640, 640, imaging.Lanczos)
	}

	var buf bytes.Buffer
	err = imaging.Encode(&buf, croppedImage, imaging.JPEG, imaging.JPEGQuality(80))
	if err != nil {
		return fmt.Errorf("failed to encode image: %v", err)
	}

	_, err = adapter.SetProfilePhoto(ctx, buf.Bytes())
	return err
}

func (service serviceUser) ChangePushName(ctx context.Context, request domainUser.ChangePushNameRequest) (err error) {
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.SetProfileName(ctx, request.PushName)
}

func (service serviceUser) IsOnWhatsApp(ctx context.Context, request domainUser.CheckRequest) (response domainUser.CheckResponse, err error) {
	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	response.IsOnWhatsApp, err = adapter.IsOnWhatsApp(ctx, request.Phone)
	return response, err
}

func (service serviceUser) BusinessProfile(ctx context.Context, request domainUser.BusinessProfileRequest) (response domainUser.BusinessProfileResponse, err error) {
	err = validations.ValidateBusinessProfile(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	profile, err := adapter.GetBusinessProfile(ctx, request.Phone)
	if err != nil {
		return response, err
	}

	response.JID = profile.JID
	response.Email = profile.Email
	response.Address = profile.Address
	response.BusinessHoursTimeZone = profile.BusinessHoursTimeZone

	for _, category := range profile.Categories {
		response.Categories = append(response.Categories, domainUser.BusinessProfileCategory{
			ID:   category.ID,
			Name: category.Name,
		})
	}

	for _, hours := range profile.BusinessHours {
		response.BusinessHours = append(response.BusinessHours, domainUser.BusinessProfileHoursConfig{
			DayOfWeek: hours.DayOfWeek,
			Mode:      hours.Mode,
			OpenTime:  hours.OpenTime,
			CloseTime: hours.CloseTime,
		})
	}

	return response, nil
}
