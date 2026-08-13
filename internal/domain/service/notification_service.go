package service

import "context"

// NotificationService defines the port for sending push notifications.
type NotificationService interface {
	// Notify sends a notification with the given title and message.
	Notify(ctx context.Context, title, message string) error
}
