package domain

// MediaIDResponse represents a response containing a media file's id and name.
type MediaIDResponse struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}
