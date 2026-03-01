package validations

import (
	"context"
	domainNewsletter "github.com/AzielCF/az-wap/core/common/channel/newsletter/domain"
	pkgError "github.com/AzielCF/az-wap/core/pkg/error"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

func ValidateUnfollowNewsletter(ctx context.Context, request domainNewsletter.UnfollowRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.NewsletterID, validation.Required),
	)

	if err != nil {
		return pkgError.ValidationError(err.Error())
	}

	return nil
}
