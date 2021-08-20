package cc_messages

type AppReschedulingRequest struct {
	Instance string `json:"instance"`
	Index    int    `json:"index"`
	CellID   string `json:"cell_id"`
	Reason   string `json:"reason,omitempty"`
}
