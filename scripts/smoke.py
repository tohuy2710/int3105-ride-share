#!/usr/bin/env python3
"""Smoke test 11 endpoints against four running services (stdlib only)."""
import json
import os
import urllib.request

bases = {name: os.environ.get(name.upper() + '_URL', 'http://localhost:' + str(8081 + i))
         for i, name in enumerate(('ride', 'location', 'matching', 'notification'))}


def check(service, method, path, status, expected, body=None):
    data = None if body is None else json.dumps(body).encode()
    request = urllib.request.Request(bases[service] + path, data=data, method=method,
                                    headers={'Content-Type': 'application/json', 'X-Request-ID': 'smoke-test'})
    with urllib.request.urlopen(request, timeout=5) as response:
        actual = json.load(response)
        assert response.status == status, (method, path, response.status)
        assert response.headers['X-Request-ID'] == 'smoke-test'
        for key, value in expected.items():
            assert actual.get(key) == value, (method, path, actual, expected)
    print('OK', service, method, path)


for name in bases:
    check(name, 'GET', '/healthz', 200, {'service': name + '-service', 'status': 'ok'})
check('ride', 'POST', '/fare', 200, {'fareId': 'fare_123', 'estimatedFare': 85000, 'etaMinutes': 12},
      {'pickupLocation': {'lat': 0, 'long': 0}, 'destination': {'lat': 21.035, 'long': 105.821}})
check('ride', 'POST', '/rides', 201, {'rideId': 'ride_123', 'status': 'REQUESTED'}, {'fareId': 'fare_123'})
check('ride', 'GET', '/rides/smoke_ride', 200, {'rideId': 'smoke_ride', 'riderId': 'rider_123', 'status': 'REQUESTED'})
check('ride', 'PATCH', '/rides/smoke_ride', 200, {'rideId': 'smoke_ride', 'action': 'ACCEPT', 'status': 'ACCEPTED'}, {'action': 'ACCEPT'})
check('ride', 'PATCH', '/rides/smoke_ride/status', 200, {'rideId': 'smoke_ride', 'status': 'COMPLETED'}, {'status': 'COMPLETED'})
check('location', 'POST', '/drivers/location', 200, {'driverId': 'driver_456', 'lat': 0, 'long': 0}, {'lat': 0, 'long': 0})
check('location', 'PATCH', '/drivers/availability', 200, {'driverId': 'driver_456', 'available': False}, {'available': False})
check('location', 'GET', '/internal/drivers/nearby?lat=0&long=0', 200,
      {'drivers': [{'driverId': 'driver_456', 'location': {'lat': 21.0285, 'long': 105.8542}, 'distanceMeters': 500}]})
check('matching', 'POST', '/internal/matching/requests', 202, {'rideId': 'smoke_ride', 'status': 'MATCHING'}, {'rideId': 'smoke_ride'})
check('matching', 'GET', '/internal/matching/requests/smoke_ride', 200, {'rideId': 'smoke_ride', 'status': 'MATCHING'})
check('notification', 'POST', '/internal/notifications/ride-requests', 202,
      {'notificationId': 'notification_123', 'rideId': 'smoke_ride', 'driverId': 'recipient', 'type': 'RIDE_REQUEST', 'status': 'QUEUED'},
      {'rideId': 'smoke_ride', 'driverId': 'recipient', 'type': 'RIDE_REQUEST'})
check('ride', 'GET', '/rides/smoke_ride', 200, {'status': 'REQUESTED'})
print('All smoke checks passed.')
