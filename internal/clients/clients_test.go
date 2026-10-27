package clients_test

import (
	"context"
	"int3105-ride-share/internal/clients"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/location"
	"int3105-ride-share/internal/matching"
	"int3105-ride-share/internal/notification"
	"reflect"
	"testing"
)

func TestClientStubsMatchServiceFixtures(t *testing.T) {
	ctx := context.Background()
	input := contracts.MatchingRequest{RideID: "custom"}
	client, _ := (clients.MatchingStub{}).Request(ctx, input)
	server, _ := (matching.MockService{}).RequestMatching(ctx, input)
	if !reflect.DeepEqual(client, server) {
		t.Fatalf("%+v != %+v", client, server)
	}
	got, _ := (clients.MatchingStub{}).Get(ctx, "custom")
	want, _ := (matching.MockService{}).GetMatching(ctx, "custom")
	if got != want {
		t.Fatalf("%+v != %+v", got, want)
	}
	nearby, _ := (clients.LocationStub{}).Nearby(ctx, contracts.NearbyInput{})
	drivers, _ := (location.MockService{}).NearbyDrivers(ctx, contracts.NearbyInput{})
	if !reflect.DeepEqual(nearby, drivers) {
		t.Fatal("location drift")
	}
	request := contracts.NotificationRequest{RideID: "custom", DriverID: "recipient", Type: "RIDE_REQUEST"}
	message, _ := (clients.NotificationStub{}).Send(ctx, request)
	fixture, _ := (notification.MockService{}).SendRideRequest(ctx, request)
	if message != fixture {
		t.Fatal("notification drift")
	}
}
