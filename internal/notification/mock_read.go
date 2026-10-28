package notification

import "context"

func (MockService) GetNotification(_ context.Context, id string) (Notification, error) {
	return Notification{ID: id, RideID: "ride_123", DriverID: "driver_456", Type: TypeRideRequest, Status: StatusQueued}, nil
}
