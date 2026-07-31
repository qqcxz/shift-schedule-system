package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type apiError struct {
	Status  int
	Message any
	Cause   error
}

func (e *apiError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprint(e.Message)
}

func badRequest(message string) error {
	return &apiError{Status: http.StatusBadRequest, Message: message}
}
func unauthorized(message string) error {
	return &apiError{Status: http.StatusUnauthorized, Message: message}
}
func forbidden(message string) error {
	return &apiError{Status: http.StatusForbidden, Message: message}
}
func serviceUnavailable(message string) error {
	return &apiError{Status: http.StatusServiceUnavailable, Message: message}
}
func notFound(message string) error { return &apiError{Status: http.StatusNotFound, Message: message} }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		apiErr = &apiError{Status: http.StatusInternalServerError, Message: "服务器内部错误", Cause: err}
	}
	label := http.StatusText(apiErr.Status)
	writeJSON(w, apiErr.Status, map[string]any{
		"message":    apiErr.Message,
		"error":      label,
		"statusCode": apiErr.Status,
	})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return badRequest("请求数据格式错误")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return badRequest("请求只能包含一个 JSON 对象")
	}
	return nil
}
