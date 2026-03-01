package domain

type LinkRequest struct {
	BaseRequest
	Caption string `json:"caption"`
	Link    string `json:"link"`
}
