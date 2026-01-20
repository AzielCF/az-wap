package domain

import "errors"

var (
	// ErrClientNotFound se retorna cuando no se encuentra un cliente
	ErrClientNotFound = errors.New("client not found")

	// ErrSubscriptionNotFound se retorna cuando no se encuentra una suscripción
	ErrSubscriptionNotFound = errors.New("subscription not found")

	// ErrDuplicateClient se retorna cuando se intenta crear un cliente duplicado
	ErrDuplicateClient = errors.New("client with this platform_id and platform_type already exists")

	// ErrDuplicateSubscription se retorna cuando se intenta crear una suscripción duplicada
	ErrDuplicateSubscription = errors.New("subscription for this client and channel already exists")

	// ErrClientDisabled se retorna cuando se intenta operar con un cliente deshabilitado
	ErrClientDisabled = errors.New("client is disabled")

	// ErrSubscriptionExpired se retorna cuando la suscripción ha expirado
	ErrSubscriptionExpired = errors.New("subscription has expired")
)
