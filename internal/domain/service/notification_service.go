package service

import "context"

type ActionType string

const (
	ActionTypeView ActionType = "view"
)

type Action struct {
	Type  ActionType
	Label string
	Url   string
	Clear bool
}

type Notification struct {
	Title   string
	Message string
	Actions []Action
	Tags    []string
}

// NotificationService defines the port for sending push notifications.
type NotificationService interface {
	// Notify sends a notification with the given title and message.
	Notify(ctx context.Context, title, message string) error
	Send(ctx context.Context, notification Notification) error
}
