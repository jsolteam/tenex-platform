package messenger

import (
	"context"
	"errors"
)

// ErrNotSupported возвращается адаптером если операция не поддерживается
// данным мессенджером. Например, VK не поддерживает EditMessage —
// хэндлер при получении этой ошибки должен отправить новое сообщение.
var ErrNotSupported = errors.New("messenger: операция не поддерживается")

// Adapter — единый интерфейс мессенджера.
// Каждый мессенджер (Telegram, VK) реализует свой адаптер.
// Бизнес-логика работает только с этим интерфейсом.
type Adapter interface {
	// Name возвращает имя мессенджера в нижнем регистре: "telegram", "vk".
	// Используется как namespace для FSM-ключей и кэша FileID.
	Name() string

	// Start запускает получение обновлений (polling или webhook).
	// Блокирует до остановки через Stop или отмены ctx.
	Start(ctx context.Context) error

	// Stop останавливает получение обновлений и освобождает ресурсы.
	Stop()

	// Updates возвращает канал входящих обновлений.
	// Канал закрывается после вызова Stop.
	Updates() <-chan *Update

	// SendMessage отправляет сообщение пользователю.
	// Возвращает SentMessage с MessageID и FileID (для фото).
	SendMessage(ctx context.Context, userID string, msg *OutMessage) (*SentMessage, error)

	// EditMessage редактирует ранее отправленное сообщение.
	// Возвращает ErrNotSupported если мессенджер не поддерживает редактирование.
	EditMessage(ctx context.Context, userID, messageID string, msg *OutMessage) error

	// DeleteMessage удаляет сообщение.
	// Возвращает nil если сообщение уже не существует.
	DeleteMessage(ctx context.Context, userID, messageID string) error

	// AnswerCallback отвечает на нажатие inline-кнопки.
	// text — всплывающее уведомление (пустая строка = тихое подтверждение).
	AnswerCallback(ctx context.Context, callbackID, text string) error
}
