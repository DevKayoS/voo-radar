package telegram

// sendMessagePayload é o corpo de POST /sendMessage da Bot API.
// chat_id aceita string numérica ("123456") ou "@canal".
type sendMessagePayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}
