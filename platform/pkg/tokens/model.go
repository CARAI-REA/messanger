package tokens

import "github.com/golang-jwt/jwt/v5"

type JoinClaims struct {
	jwt.RegisteredClaims
	UserUUID string `json:"user_id"`
	RoomUUID string `json:"room_uuid"`
}

// AccessClaims — claims JWT от API Gateway (room_task.md §5.3, §7.5).
type AccessClaims struct {
	jwt.RegisteredClaims
	UserUUID string `json:"user_id"`
}

type JoinInfo interface {
	GetUserUUID() string
	GetRoomUUID() string
}
