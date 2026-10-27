#!/usr/bin/env python3
"""Real authenticated Ride/Auth flow and optional mock-service contract smoke."""
import argparse
import json
import os
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('--core-only', action='store_true', help='only Auth and Ride (Kaggle)')
args = parser.parse_args()
bases = {name: os.environ.get(name.upper() + '_URL', 'http://localhost:' + str(8081 + i))
         for i, name in enumerate(('ride', 'location', 'matching', 'notification', 'auth'))}
internal = os.environ['INTERNAL_TOKEN']


def check(service, method, path, status, expected=None, body=None, token=None, internal_token=None):
    data = None if body is None else json.dumps(body).encode()
    headers = {'Content-Type': 'application/json', 'X-Request-ID': 'smoke-test'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if internal_token:
        headers['X-Internal-Token'] = internal_token
    request = urllib.request.Request(bases[service] + path, data=data, method=method, headers=headers)
    try:
        response = urllib.request.urlopen(request, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        actual = json.load(response)
        assert response.status == status, (method, path, response.status, actual)
        assert response.headers['X-Request-ID'] == 'smoke-test'
        for key, value in (expected or {}).items():
            assert actual.get(key) == value, (method, path, actual, expected)
    print('OK', service, method, path, status)
    return actual


for name in ('ride', 'auth') if args.core_only else bases:
    check(name, 'GET', '/healthz', 200, {'service': name + '-service', 'status': 'ok'})
    check(name, 'GET', '/readyz', 200)
password = os.environ.get('FIXTURE_PASSWORD', 'RideShare123!')

def login(email):
    return check('auth', 'POST', '/auth/login', 200, body={'email': email, 'password': password})['accessToken']

rider, driver, other = login('rider@example.com'), login('driver1@example.com'), login('driver2@example.com')
check('auth', 'GET', '/auth/me', 200, {'userId': 'rider_123', 'role': 'RIDER'}, token=rider)
check('auth', 'POST', '/auth/register', 201, {'userId': 'user_mock', 'persisted': False},
      {'email': 'new@example.com', 'password': password, 'role': 'RIDER'})
check('auth', 'POST', '/auth/login', 401, body={'email': 'new@example.com', 'password': password})
check('ride', 'POST', '/rides', 401, body={'fareId': 'fare_123'})
check('ride', 'GET', '/rides/missing', 401)
check('ride', 'POST', '/rides', 403, body={'fareId': 'fare_123'}, token=driver)
check('ride', 'POST', '/fare', 200, {'fareId': 'fare_123', 'estimatedFare': 85000, 'etaMinutes': 12},
      {'pickupLocation': {'lat': 0, 'long': 0}, 'destination': {'lat': 21.035, 'long': 105.821}}, token=rider)
ride = check('ride', 'POST', '/rides', 201, {'status': 'REQUESTED'}, {'fareId': 'fare_123'}, token=rider)['rideId']
path = '/rides/' + ride
check('ride', 'GET', path, 200, {'riderId': 'rider_123', 'driverId': None, 'status': 'REQUESTED'}, token=rider)
check('ride', 'GET', '/internal/rides/' + ride, 401)
check('ride', 'GET', '/internal/rides/' + ride, 200, {'rideId': ride}, internal_token=internal)
check('ride', 'PATCH', path, 200, {'driverId': None, 'status': 'REQUESTED'}, {'action': 'DENY'}, token=other)
check('ride', 'PATCH', path, 200, {'driverId': 'driver_456', 'status': 'ACCEPTED'}, {'action': 'ACCEPT'}, token=driver)
check('ride', 'PATCH', path, 200, {'driverId': 'driver_456', 'status': 'ACCEPTED'}, {'action': 'ACCEPT'}, token=driver)
check('ride', 'PATCH', path, 409, body={'action': 'ACCEPT'}, token=other)
check('ride', 'GET', path, 403, token=other)
check('ride', 'GET', path, 200, {'driverId': 'driver_456', 'status': 'ACCEPTED'}, token=driver)
check('ride', 'DELETE', path, 409, token=rider)
check('ride', 'PATCH', path + '/status', 409, body={'status': 'COMPLETED'}, token=driver)
for status in ('ARRIVED', 'IN_PROGRESS', 'COMPLETED'):
    check('ride', 'PATCH', path + '/status', 200, {'status': status}, {'status': status}, token=driver)
    check('ride', 'PATCH', path + '/status', 200, {'status': status}, {'status': status}, token=driver)
check('ride', 'GET', path, 200, {'status': 'COMPLETED'}, token=rider)
cancel = check('ride', 'POST', '/rides', 201, body={'fareId': 'fare_123'}, token=rider)['rideId']
for _ in range(2):
    check('ride', 'DELETE', '/rides/' + cancel, 200, {'status': 'CANCELLED'}, token=rider)
check('ride', 'PATCH', '/rides/' + cancel, 409, body={'action': 'ACCEPT'}, token=driver)
check('ride', 'GET', '/rides/missing', 404, token=rider)

if not args.core_only:
    check('location', 'POST', '/drivers/location', 200, {'driverId': 'driver_456', 'lat': 0, 'long': 0},
          {'lat': 0, 'long': 0}, token=driver)
    check('location', 'PATCH', '/drivers/availability', 200, {'driverId': 'driver_456', 'available': False},
          {'available': False}, token=driver)
    check('location', 'GET', '/internal/drivers/nearby?lat=0&long=0', 200,
          {'drivers': [{'driverId': 'driver_456', 'location': {'lat': 21.0285, 'long': 105.8542}, 'distanceMeters': 500}]}, internal_token=internal)
    check('matching', 'POST', '/internal/matching/requests', 202, {'rideId': ride, 'status': 'MATCHING'},
          {'rideId': ride}, internal_token=internal)
    check('matching', 'GET', '/internal/matching/requests/' + ride, 200, {'rideId': ride, 'status': 'MATCHING'}, internal_token=internal)
    check('notification', 'POST', '/internal/notifications/ride-requests', 202,
          {'notificationId': 'notification_123', 'rideId': ride, 'driverId': 'driver_456', 'type': 'RIDE_REQUEST', 'status': 'QUEUED'},
          {'rideId': ride, 'driverId': 'driver_456', 'type': 'RIDE_REQUEST'}, internal_token=internal)
print('All authenticated smoke checks passed.')
