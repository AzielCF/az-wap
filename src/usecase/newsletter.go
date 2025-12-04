package usecase

import (
	"context"

	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	"github.com/AzielCF/az-wap/infrastructure/whatsapp"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/validations"
)

type serviceNewsletter struct{}

func NewNewsletterService() domainNewsletter.INewsletterUsecase {
	return &serviceNewsletter{}
}

func (service serviceNewsletter) Unfollow(ctx context.Context, request domainNewsletter.UnfollowRequest) (err error) {
	if err = validations.ValidateUnfollowNewsletter(ctx, request); err != nil {
		return err
	}

	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return err
	}

	return whatsapp.GetClient().UnfollowNewsletter(ctx, JID)
}
