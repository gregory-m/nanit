package baby

// Baby - baby info (matching the Nanit API)
type Baby struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	CameraUID string `json:"camera_uid"`
}

// Message - message info (matching the Nanit API)
type Message struct {
	UID  string `json:"baby_uid"`
	Type string `json:"type"`
	Time int    `json:"time"`
	// More..
}
