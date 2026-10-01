package tokens

import "errors"

// ErrTokenExpired is returned when a join/access JWT is past its exp claim.
var ErrTokenExpired = errors.New("token expired")
