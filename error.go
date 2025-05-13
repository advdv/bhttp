package bhttp

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is an error code that mirrors the http status codes. It can be used to create errors to pass around across
// middleware layers to handle errors structurally.
type Code int

const (
	CodeUnknown                      Code = 0
	CodeBadRequest                   Code = http.StatusBadRequest                   // RFC 9110, 15.5.1
	CodeUnauthorized                 Code = http.StatusUnauthorized                 // RFC 9110, 15.5.2
	CodePaymentRequired              Code = http.StatusPaymentRequired              // RFC 9110, 15.5.3
	CodeForbidden                    Code = http.StatusForbidden                    // RFC 9110, 15.5.4
	CodeNotFound                     Code = http.StatusNotFound                     // RFC 9110, 15.5.5
	CodeMethodNotAllowed             Code = http.StatusMethodNotAllowed             // RFC 9110, 15.5.6
	CodeNotAcceptable                Code = http.StatusNotAcceptable                // RFC 9110, 15.5.7
	CodeProxyAuthRequired            Code = http.StatusProxyAuthRequired            // RFC 9110, 15.5.8
	CodeRequestTimeout               Code = http.StatusRequestTimeout               // RFC 9110, 15.5.9
	CodeConflict                     Code = http.StatusConflict                     // RFC 9110, 15.5.10
	CodeGone                         Code = http.StatusGone                         // RFC 9110, 15.5.11
	CodeLengthRequired               Code = http.StatusLengthRequired               // RFC 9110, 15.5.12
	CodePreconditionFailed           Code = http.StatusPreconditionFailed           // RFC 9110, 15.5.13
	CodeRequestEntityTooLarge        Code = http.StatusRequestEntityTooLarge        // RFC 9110, 15.5.14
	CodeRequestURITooLong            Code = http.StatusRequestURITooLong            // RFC 9110, 15.5.15
	CodeUnsupportedMediaType         Code = http.StatusUnsupportedMediaType         // RFC 9110, 15.5.16
	CodeRequestedRangeNotSatisfiable Code = http.StatusRequestedRangeNotSatisfiable // RFC 9110, 15.5.17
	CodeExpectationFailed            Code = http.StatusExpectationFailed            // RFC 9110, 15.5.18
	CodeTeapot                       Code = http.StatusTeapot                       // RFC 9110, 15.5.19 (Unused)
	CodeMisdirectedRequest           Code = http.StatusMisdirectedRequest           // RFC 9110, 15.5.20
	CodeUnprocessableEntity          Code = http.StatusUnprocessableEntity          // RFC 9110, 15.5.21
	CodeLocked                       Code = http.StatusLocked                       // RFC 4918, 11.3
	CodeFailedDependency             Code = http.StatusFailedDependency             // RFC 4918, 11.4
	CodeTooEarly                     Code = http.StatusTooEarly                     // RFC 8470, 5.2.
	CodeUpgradeRequired              Code = http.StatusUpgradeRequired              // RFC 9110, 15.5.22
	CodePreconditionRequired         Code = http.StatusPreconditionRequired         // RFC 6585, 3
	CodeTooManyRequests              Code = http.StatusTooManyRequests              // RFC 6585, 4
	CodeRequestHeaderFieldsTooLarge  Code = http.StatusRequestHeaderFieldsTooLarge  // RFC 6585, 5
	CodeUnavailableForLegalReasons   Code = http.StatusUnavailableForLegalReasons   // RFC 7725, 3

	CodeInternalServerError           Code = http.StatusInternalServerError           // RFC 9110, 15.6.1
	CodeNotImplemented                Code = http.StatusNotImplemented                // RFC 9110, 15.6.2
	CodeBadGateway                    Code = http.StatusBadGateway                    // RFC 9110, 15.6.3
	CodeServiceUnavailable            Code = http.StatusServiceUnavailable            // RFC 9110, 15.6.4
	CodeGatewayTimeout                Code = http.StatusGatewayTimeout                // RFC 9110, 15.6.5
	CodeHTTPVersionNotSupported       Code = http.StatusHTTPVersionNotSupported       // RFC 9110, 15.6.6
	CodeVariantAlsoNegotiates         Code = http.StatusVariantAlsoNegotiates         // RFC 2295, 8.1
	CodeInsufficientStorage           Code = http.StatusInsufficientStorage           // RFC 4918, 11.5
	CodeLoopDetected                  Code = http.StatusLoopDetected                  // RFC 5842, 7.2
	CodeNotExtended                   Code = http.StatusNotExtended                   // RFC 2774, 7
	CodeNetworkAuthenticationRequired Code = http.StatusNetworkAuthenticationRequired // RFC 6585, 6
)

// Error describes an http error.
type Error struct {
	code Code
	err  error
}

// NewError inits a new error given the error code.
func NewError(c Code, underlying error) *Error {
	return &Error{c, underlying}
}

func (e *Error) Code() Code { return e.code }
func (e *Error) Error() string {
	status := http.StatusText(int(e.Code()))
	if status == "" {
		status = "Unknown"
	}

	return fmt.Sprintf("%s: %s", status, e.err.Error())
}

// CodeOf returns the error's status code if it is or wraps an [*Error] and
// [CodeUnknown] otherwise.
func CodeOf(err error) Code {
	if connectErr, ok := asError(err); ok {
		return connectErr.Code()
	}
	return CodeUnknown
}

// asError uses errors.As to unwrap any error and look for a connect *Error.
func asError(err error) (*Error, bool) {
	var connectErr *Error
	ok := errors.As(err, &connectErr)
	return connectErr, ok
}
