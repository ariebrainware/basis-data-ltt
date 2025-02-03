package model

type Patient struct {
	FullName      string `json:"full_name"`
	Gender        string `json:"gender"`
	Age           int    `json:"age"`
	Job           string `json:"job"`
	Address       string `json:"address"`
	PhoneNumber   string `json:"phone_number"`
	HealthHistory []uint `json:"health_history"`
}
