package types

//-------- Queries --------

type QueryResponse struct {
	Ok  []byte    `json:"Ok,omitempty"`
	Err *ApiError `json:"Err,omitempty"`
}
