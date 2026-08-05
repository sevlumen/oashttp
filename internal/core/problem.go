package core

type ProblemDetails struct {
	Type     string              `json:"type,omitempty"`
	Title    string              `json:"title"`
	Status   int                 `json:"status"`
	Detail   string              `json:"detail,omitempty"`
	Instance string              `json:"instance,omitempty"`
	Code     string              `json:"code,omitempty"`
	TraceID  string              `json:"traceId,omitempty"`
	Errors   map[string][]string `json:"errors,omitempty"`
}
