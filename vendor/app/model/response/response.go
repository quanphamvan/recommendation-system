package response

type ResponseData struct {
	Recommends []Post `json:"recommend,omitempty"`
	Algorithm  int      `json:"alg,omitempty"`
}

type Post struct {
	ID int64 `json:"id,omitempty"`
}