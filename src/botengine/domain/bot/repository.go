package bot

import (
	"context"
)

// IBotRepository define el contrato para el acceso a datos de bots.
// Ubicado en el dominio para seguir los principios de Clean Architecture (DIP).
type IBotRepository interface {
	// Init inicializa el esquema de la base de datos (tablas, migraciones).
	Init(ctx context.Context) error

	// Create inserta un nuevo bot en la base de datos.
	Create(ctx context.Context, bot Bot) error

	// GetByID obtiene un bot por su ID.
	GetByID(ctx context.Context, id string) (Bot, error)

	// List retorna todos los bots ordenados por nombre.
	List(ctx context.Context) ([]Bot, error)

	// Update actualiza un bot existente.
	Update(ctx context.Context, bot Bot) error

	// Delete elimina un bot por su ID.
	Delete(ctx context.Context, id string) error
}
