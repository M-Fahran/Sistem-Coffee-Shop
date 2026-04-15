package exception

// Exception represents a structured HTTP error.
//
// Services construct exceptions directly and return them via c.Error().
// The ErrorHandler middleware serializes them into a consistent JSON envelope.
type Exception struct {
	Code      int    `json:"-"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	Err       error  `json:"-"`
}

func (e *Exception) Error() string {
	return e.Message
}

func (e *Exception) Unwrap() error {
	return e.Err
}

// WithDetails attaches extra context to the exception. Returns the same
// pointer for chaining.
//
//	exception.Conflict("BRANCH_409", "branch has employees").WithDetails(gin.H{"id": id})
func (e *Exception) WithDetails(d any) *Exception {
	e.Details = d
	return e
}

// --------- base constructors ---------

// New creates an Exception with HTTP status code, error code, and message.
func New(code int, errCode, msg string) *Exception {
	return &Exception{
		Code:      code,
		ErrorCode: errCode,
		Message:   msg,
	}
}

// Wrap creates an Exception that wraps an underlying error for logging.
// The underlying error is never exposed to the client.
func Wrap(err error, code int, errCode, msg string) *Exception {
	return &Exception{
		Code:      code,
		ErrorCode: errCode,
		Message:   msg,
		Err:       err,
	}
}