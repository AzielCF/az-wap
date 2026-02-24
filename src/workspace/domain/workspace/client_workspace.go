package workspace

import "time"

// ClientWorkspace es el agrupador lógico del cliente (ej: "Sucursal Norte").
type ClientWorkspace struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"` // Client dueño
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ClientWorkspaceChannel enlaza un Workspace con un Canal real.
type ClientWorkspaceChannel struct {
	ClientWorkspaceID string    `json:"client_workspace_id"`
	ChannelID         string    `json:"channel_id"`
	CreatedAt         time.Time `json:"created_at"`
}

// ClientWorkspaceGuest representa al invitado que puede acceder a los bots.
type ClientWorkspaceGuest struct {
	ID                  string              `json:"id"`
	OwnerID             string              `json:"owner_id"` // El cliente dueño
	ClientWorkspaceID   string              `json:"client_workspace_id"`
	Name                string              `json:"name"`
	BotID               string              `json:"bot_id"`
	BotTemplateID       string              `json:"bot_template_id"` // ID del Bot (o Bot:Variant)
	PlatformIdentifiers PlatformIdentifiers `json:"platform_identifiers"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

// PlatformIdentifiers contiene los IDs por plataforma (ej: {"whatsapp": "+123456", "telegram": "pedro_caja"}).
type PlatformIdentifiers map[string]string
