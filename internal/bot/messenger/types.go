package messenger

// ParseMode — режим форматирования текста сообщения.
type ParseMode string

const (
	ParseModeHTML     ParseMode = "HTML"
	ParseModeMarkdown ParseMode = "Markdown"
	ParseModePlain    ParseMode = ""
)

// OutMessage — исходящее сообщение от бота пользователю.
// Поля фото применяются по приоритету: FileID → PhotoData → PhotoURL.
type OutMessage struct {
	// Текст сообщения.
	Text string

	// ParseMode — режим разметки текста.
	ParseMode ParseMode

	// Keyboard — встроенная или reply-клавиатура. Nil = не менять.
	Keyboard *Keyboard

	// PhotoFileID — закэшированный FileID; отправляется первым если задан.
	PhotoFileID string

	// PhotoData — бинарные данные фото; используется если FileID пуст.
	PhotoData []byte

	// PhotoContentType — MIME-тип для PhotoData (напр. "image/jpeg").
	PhotoContentType string

	// PhotoURL — публичный URL фото; используется если FileID и PhotoData пусты.
	PhotoURL string

	// ReplyToMessageID — ID сообщения на которое отвечаем (опционально).
	ReplyToMessageID string
}

// SentMessage — результат успешной отправки сообщения.
type SentMessage struct {
	// MessageID — ID отправленного сообщения в мессенджере.
	MessageID string

	// FileID — FileID фото в мессенджере после первой отправки.
	// Непустой только при отправке фото. Используется для кэширования.
	FileID string
}

// Update — входящее событие от пользователя (сообщение или нажатие кнопки).
type Update struct {
	// UserID — идентификатор пользователя в мессенджере.
	UserID string

	// MessageID — ID сообщения (для удаления/редактирования).
	MessageID string

	// Text — текст сообщения или подпись фото.
	Text string

	// CallbackData — данные нажатой inline-кнопки.
	CallbackData string

	// CallbackID — ID callback-запроса (для AnswerCallback).
	CallbackID string

	// Photo — бинарные данные фото если пользователь отправил фото.
	Photo []byte

	// PhotoFileID — FileID фото в мессенджере (для кэширования).
	PhotoFileID string

	// Language — язык пользователя из профиля мессенджера (напр. "ru", "en").
	Language string
}

// IsCallback возвращает true если Update — нажатие inline-кнопки.
func (u *Update) IsCallback() bool {
	return u.CallbackData != ""
}

// IsPhoto возвращает true если Update содержит фото.
func (u *Update) IsPhoto() bool {
	return len(u.Photo) > 0 || u.PhotoFileID != ""
}
