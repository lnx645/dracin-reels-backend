package domain

// AppError is a business error with an HTTP status and a stable code.
// Messages are in Indonesian because they are shown to end users.
type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e *AppError) Error() string { return e.Message }

func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func ErrNotFound(what string) *AppError {
	return NewError(404, "NOT_FOUND", what+" tidak ditemukan.")
}

func ErrUnauthorized(msg string) *AppError {
	return NewError(401, "UNAUTHORIZED", msg)
}

func ErrForbidden(msg string) *AppError {
	return NewError(403, "FORBIDDEN", msg)
}

func ErrBadRequest(code, msg string) *AppError {
	return NewError(400, code, msg)
}

func ErrConflict(code, msg string) *AppError {
	return NewError(409, code, msg)
}
