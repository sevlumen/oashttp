package core

// Failure is the framework-neutral description of a failed request.
// A FailureFormatter converts it into the application's public error body.
type Failure struct {
	Type     string
	Title    string
	Status   int
	Detail   string
	Instance string
	Code     string
	TraceID  string
	Errors   map[string][]string
}

// FailureFormatter controls the content type, OpenAPI schema, and runtime body
// used for framework-generated failures and Fail results.
type FailureFormatter interface {
	ContentType() string
	Model() any
	Format(Failure) any
}

// ProblemDetailsFormatter is the default RFC 9457-compatible formatter.
type ProblemDetailsFormatter struct{}

func (ProblemDetailsFormatter) ContentType() string { return "application/problem+json" }
func (ProblemDetailsFormatter) Model() any          { return ProblemDetails{} }
func (ProblemDetailsFormatter) Format(f Failure) any {
	return ProblemDetails{
		Type:     f.Type,
		Title:    f.Title,
		Status:   f.Status,
		Detail:   f.Detail,
		Instance: f.Instance,
		Code:     f.Code,
		TraceID:  f.TraceID,
		Errors:   f.Errors,
	}
}
