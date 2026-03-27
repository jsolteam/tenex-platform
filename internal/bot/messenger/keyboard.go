package messenger

// KbType — тип клавиатуры.
type KbType int

const (
	// KbInline — встроенная клавиатура под сообщением (inline keyboard).
	KbInline KbType = iota

	// KbReply — клавиатура вместо стандартной (reply keyboard).
	KbReply

	// KbRemove — удалить reply-клавиатуру.
	KbRemove
)

// Button — одна кнопка клавиатуры.
type Button struct {
	// Text — отображаемый текст кнопки.
	Text string

	// Data — callback_data для inline-кнопок (роутинг в боте).
	Data string

	// Selected — кнопка отмечена (добавляет ✓ к тексту при отображении).
	Selected bool

	// URL — ссылка для inline-кнопок типа url (опционально).
	URL string
}

// Keyboard — абстрактная клавиатура, независимая от мессенджера.
// Адаптер конвертирует её в нативный формат перед отправкой.
type Keyboard struct {
	Type KbType
	Rows [][]Button
}

// ── конструкторы клавиатур ────────────────────────────────────────────────────

// InlineKb создаёт встроенную клавиатуру из готовых строк.
func InlineKb(rows ...[]Button) *Keyboard {
	return &Keyboard{Type: KbInline, Rows: rows}
}

// ReplyKb создаёт reply-клавиатуру из готовых строк.
func ReplyKb(rows ...[]Button) *Keyboard {
	return &Keyboard{Type: KbReply, Rows: rows}
}

// RemoveKb возвращает команду удалить reply-клавиатуру.
func RemoveKb() *Keyboard {
	return &Keyboard{Type: KbRemove}
}

// ── конструкторы строк и кнопок ───────────────────────────────────────────────

// Row создаёт строку кнопок.
//
//	Row(Btn("Да", "confirm:yes"), Btn("Нет", "confirm:no"))
func Row(buttons ...Button) []Button {
	return buttons
}

// Btn создаёт обычную кнопку.
//
//	Btn("Добавить лекарство", "med:add")
func Btn(text, data string) Button {
	return Button{Text: text, Data: data}
}

// BtnURL создаёт кнопку-ссылку.
func BtnURL(text, url string) Button {
	return Button{Text: text, URL: url}
}

// BtnSelected создаёт кнопку с отметкой ✓.
// Используется в DaysOfWeek и TimePicker для показа выбранных значений.
//
//	BtnSelected("Понедельник", "day:1")  →  "✓ Понедельник"
func BtnSelected(text, data string) Button {
	return Button{Text: "✓ " + text, Data: data, Selected: true}
}

// BtnToggle возвращает BtnSelected если selected == true, иначе Btn.
// Удобно при построении динамических клавиатур с мультивыбором.
//
//	BtnToggle("Пн", "day:1", weekdays.Has(Monday))
func BtnToggle(text, data string, selected bool) Button {
	if selected {
		return BtnSelected(text, data)
	}
	return Btn(text, data)
}
