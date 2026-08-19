package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}
type Handler func(http.ResponseWriter, *http.Request) error
type ContextKey string

const RequestIDKey ContextKey = "request_id"

func RequestID(r *http.Request) string { v, _ := r.Context().Value(RequestIDKey).(string); return v }
func NewRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = NewRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), RequestIDKey, id)))
	})
}
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func Decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	d := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if d.More() {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
func Adapt(fn Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal"
	switch {
	case errors.Is(err, ErrBadRequest):
		status = http.StatusBadRequest
		code = "invalid_argument"
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
		code = "conflict"
	case errors.Is(err, ErrPrecondition):
		status = http.StatusPreconditionFailed
		code = "precondition_failed"
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
		code = "forbidden"
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
		code = "deadline_exceeded"
	}
	JSON(w, status, Error{Code: code, Message: err.Error(), RequestID: RequestID(r)})
}

var (
	ErrBadRequest   = errors.New("bad request")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrPrecondition = errors.New("precondition failed")
	ErrForbidden    = errors.New("forbidden")
)

func Page(r *http.Request) (int, int) {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	s, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if p < 1 {
		p = 1
	}
	if s < 1 {
		s = 50
	}
	if s > 200 {
		s = 200
	}
	return p, s
}
func ETag(version int64) string { return `\"` + strconv.FormatInt(version, 10) + `\"` }
func MatchETag(r *http.Request, version int64) bool {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	return raw == "*" || raw == ETag(version)
}
