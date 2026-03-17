package user

import "context"

type Repository interface {
	// GetByID возвращает пользователя по его ID.
	GetByID(ctx context.Context, id int64) (*User, error)

	// GetByMessenger возвращает пользователя по идентификатору в мессенджере.
	GetByMessenger(ctx context.Context, messengerType MessengerType, messengerUserID string) (*User, error)

	// Create создаёт нового пользователя вместе с первичным контактом.
	Create(ctx context.Context, user *User, contact *UserContact) error

	// Update обновляет timezone и language пользователя.
	Update(ctx context.Context, user *User) error

	// GetContacts возвращает все контакты пользователя.
	GetContacts(ctx context.Context, userID int64) ([]UserContact, error)

	// AddContact добавляет новый контакт пользователю.
	AddContact(ctx context.Context, contact *UserContact) error

	// SetPrimaryContact делает указанный контакт основным.
	SetPrimaryContact(ctx context.Context, userID int64, contactID int64) error
}

type StatisticsRepository interface {
	// GetByUserID возвращает статистику пользователя.
	GetByUserID(ctx context.Context, userID int64) (*UserStatistics, error)

	// Upsert создаёт или обновляет статистику пользователя.
	Upsert(ctx context.Context, stats *UserStatistics) error
}
