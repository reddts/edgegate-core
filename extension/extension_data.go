package extension

type extensionData struct {
	Id       string `json:"id"`
	Enable   bool   `json:"enable"`
	JsonData []byte
}
