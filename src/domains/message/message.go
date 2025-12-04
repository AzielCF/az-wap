package message

type GenericResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

type RevokeRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	Token     string `json:"token,omitempty" form:"token"`
}

type DeleteRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	Token     string `json:"token,omitempty" form:"token"`
}

type ReactionRequest struct {
	MessageID string `json:"message_id" form:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	Emoji     string `json:"emoji" form:"emoji"`
	Token     string `json:"token,omitempty" form:"token"`
}

type UpdateMessageRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Message   string `json:"message" form:"message"`
	Phone     string `json:"phone" form:"phone"`
	Token     string `json:"token,omitempty" form:"token"`
}

type MarkAsReadRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	Token     string `json:"token,omitempty" form:"token"`
}

type StarRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	IsStarred bool   `json:"is_starred"`
	Token     string `json:"token,omitempty" form:"token"`
}

type DownloadMediaRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	Token     string `json:"token,omitempty" form:"token"`
}

type DownloadMediaResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	MediaType string `json:"media_type"`
	Filename  string `json:"filename"`
	FilePath  string `json:"file_path"`
	FileSize  int64  `json:"file_size"`
}
