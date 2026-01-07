package usecase

import (
	"context"

	domainInstance "github.com/AzielCF/az-wap/domains/instance"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	"github.com/AzielCF/az-wap/infrastructure/whatsapp"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/validations"
)

type serviceNewsletter struct {
	instanceService domainInstance.IInstanceUsecase
}

func NewNewsletterService(instanceService domainInstance.IInstanceUsecase) domainNewsletter.INewsletterUsecase {
	return &serviceNewsletter{instanceService: instanceService}
}

func (service serviceNewsletter) Unfollow(ctx context.Context, request domainNewsletter.UnfollowRequest) (err error) {
	if err = validations.ValidateUnfollowNewsletter(ctx, request); err != nil {
		return err
	}

	// Newsletters currently use the global client as they don't have token support yet
	client := whatsapp.GetClient()
	JID, err := utils.ValidateJidWithLogin(client, request.NewsletterID)
	if err != nil {
		return err
	}

	return client.UnfollowNewsletter(ctx, JID)
}
