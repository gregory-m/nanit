package message

// Message - message info (matching the Nanit API)
type Message struct {
	Id      int    `json:"id"`
	BabyUid string `json:"baby_uid"`
	UserId  int    `json:"user_id"`
	Type    string `json:"type"`
	Time    int    `json:"time"`
	// Format for ReadAt, SeenAt, and DismissedAt TBD
	ReadAt      string `json:"read_at"`
	SeenAt      string `json:"seen_at"`
	DismissedAt string `json:"dismissed_at"`
	// TODO: unmarshall ISO8601 timestamp into time.Time for UpdatedAt and CreatedAt
	UpdatedAt string `json:"updated_at"`
	CreatedAt string `json:"created_at"`
	// TODO: enumerate possible Data interface structures
	Data interface{} `json:"data"`
}
