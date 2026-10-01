package turncred

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

// Credential is a time-limited TURN username/password for coturn use-auth-secret.
type Credential struct {
	Username string
	Password string
	TTL      time.Duration
}

// Generate HMAC credentials: username = "<expiry>:<userID>", password = base64(HMAC-SHA1(secret, username)).
func Generate(sharedSecret, userID string, ttl time.Duration) (Credential, error) {
	if sharedSecret == "" {
		return Credential{}, fmt.Errorf("turn shared secret is empty")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	expiry := time.Now().Add(ttl).Unix()
	username := strconv.FormatInt(expiry, 10) + ":" + userID
	mac := hmac.New(sha1.New, []byte(sharedSecret))
	_, _ = mac.Write([]byte(username))
	password := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return Credential{Username: username, Password: password, TTL: ttl}, nil
}
