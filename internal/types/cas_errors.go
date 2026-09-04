package types

import "errors"

// CAS cookie dual-channel resolve sentinels (middleware uses errors.Is).
var (
	ErrCASCredentialsMissing    = errors.New("cas credentials missing")
	ErrCASTicketInvalid         = errors.New("cas ticket invalid")
	ErrCASUserCenterUnavailable = errors.New("cas user center unavailable")
)
