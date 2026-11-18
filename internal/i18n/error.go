package i18n

import (
	"errors"

	"github.com/gin-gonic/gin"
)

// Error carries an i18n message key and template arguments while implementing
// the error interface. Error() returns Fallback when set (preserving legacy
// error-string contracts in logs/tests), otherwise the DefaultLang translation.
type Error struct {
	Key      string
	Params   map[string]any
	Fallback string
	Cause    error
}

// NewError creates a translatable i18n Error with optional template parameters.
func NewError(key string, params ...map[string]any) *Error {
	var p map[string]any
	if len(params) > 0 {
		p = params[0]
	}
	return &Error{
		Key:    key,
		Params: p,
	}
}

// NewErrorWithFallback creates a translatable Error while preserving a fixed
// fallback string for Error() callers and unit tests.
func NewErrorWithFallback(fallback, key string, params ...map[string]any) *Error {
	e := NewError(key, params...)
	e.Fallback = fallback
	return e
}

// WithCause attaches an underlying cause error and returns the receiver.
func (e *Error) WithCause(err error) *Error {
	e.Cause = err
	return e
}

// Error implements error using Fallback if non-empty, otherwise DefaultLang translation.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Fallback != "" {
		return e.Fallback
	}
	return Translate(DefaultLang, e.Key, e.Params)
}

// Unwrap returns the underlying cause error if present.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// TranslateLocalized resolves the error into lang, recursively translating nested
// *i18n.Error parameters or Cause errors.
func (e *Error) TranslateLocalized(lang string) string {
	if e == nil {
		return ""
	}
	if len(e.Params) == 0 && e.Cause == nil {
		return Translate(lang, e.Key, nil)
	}
	params := make(map[string]any, len(e.Params)+1)
	for k, v := range e.Params {
		switch tv := v.(type) {
		case *Error:
			params[k] = tv.TranslateLocalized(lang)
		case error:
			params[k] = ResolveError(lang, tv, "")
		default:
			params[k] = v
		}
	}
	if e.Cause != nil {
		if _, hasErr := params["Error"]; !hasErr {
			params["Error"] = ResolveError(lang, e.Cause, "")
		}
	}
	return Translate(lang, e.Key, params)
}

// ResolveError translates err into lang if err wraps an *i18n.Error.
// If fallbackKey is non-empty and err does not wrap an *i18n.Error,
// Translate(lang, fallbackKey) is returned instead of leaking raw err.Error().
func ResolveError(lang string, err error, fallbackKey string) string {
	if err == nil {
		return ""
	}
	var localizedErr *Error
	if errors.As(err, &localizedErr) {
		return localizedErr.TranslateLocalized(lang)
	}
	if fallbackKey != "" {
		return Translate(lang, fallbackKey)
	}
	return err.Error()
}

// ResolveContextError resolves err using the request context's preferred language.
func ResolveContextError(c *gin.Context, err error, fallbackKey string) string {
	return ResolveError(GetLangFromContext(c), err, fallbackKey)
}
