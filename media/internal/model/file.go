package model

import "time"

type File struct {
	ID        string
	OwnerID   int64
	ObjectKey string
	Filename  string
	Mime      string
	SizeBytes int64
	Status    string
	CreatedAt time.Time
}
