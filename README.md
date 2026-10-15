# Ride Share — Go/Gin mock backend

Bốn service độc lập, một Go module `int3105-ride-share`, đủ 11 endpoint đặt xe. Go **1.25.9**, Gin **v1.12.0**. Chỉ validate input và trả JSON mock; chưa có logic nghiệp vụ.

## Chạy bằng Go

Cần Go 1.25.9 và Make. Tải dependency:

```bash
go mod download
```

Chạy mỗi lệnh trong một terminal riêng:

```bash
make run-ride
make run-location
make run-matching
make run-notification
```

| Service | Port | OpenAPI |
| --- | --- | --- |
| Ride | 8081 | [ride.yaml](docs/openapi/ride.yaml) |
| Location | 8082 | [location.yaml](docs/openapi/location.yaml) |
| Matching | 8083 | [matching.yaml](docs/openapi/matching.yaml) |
| Notification | 8084 | [notification.yaml](docs/openapi/notification.yaml) |

Mỗi tiến trình đọc `PORT`, `GIN_MODE` (`debug`, `release`, `test`) và `LOG_LEVEL` (`debug`, `info`, `warn`, `error`). Mặc định dùng port trong bảng, mode `release`, log level `info`.

```bash
PORT=9081 GIN_MODE=debug LOG_LEVEL=debug make run-ride
```

Các lệnh Go không tự đọc `.env`. `RIDE_PORT`, `LOCATION_PORT`, `MATCHING_PORT`, `NOTIFICATION_PORT` trong `.env.example` là **host port cho Docker Compose**.

## Chạy bằng Docker

Cần Docker Engine và Docker Compose hỗ trợ `--wait`:

```bash
cp .env.example .env
make compose-up
make compose-down
```

`compose-up` build bốn image từ Dockerfile dùng chung với build arg `SERVICE`, đợi health check thành công. Mỗi image chạy một binary bằng user không phải root. Không có gateway: gọi trực tiếp port của từng service. Khi đổi host port trong `.env`, đổi URL curl/OpenAPI tương ứng.

## Cấu trúc và mở rộng

```text
cmd/{ride,location,matching,notification}/main.go
internal/{ride,location,matching,notification}/
    router.go     # Route và identity middleware
    handler.go    # Bind/validate, gọi Service, trả HTTP response
    dto.go        # Request/response contract
    service.go    # Interface Service và MockService stateless
internal/platform/
    api/          # Validation, error envelope, request ID, log, recovery
    app/          # Config, HTTP server, graceful shutdown
```

Để thêm logic thật, tạo implementation mới của interface `Service` và thay `MockService{}` ở entrypoint. Handler và router không cần thay đổi. Chưa có repository layer, DB, Redis, Kafka, gateway, JWT hoặc push provider.

## Thử đủ 11 endpoint

### Ride (8081)

```bash
# 1. Ước tính giá → 200: fare_123, 85000 VND, ETA 12 phút
curl -i http://localhost:8081/fare -H 'Content-Type: application/json' \
  -d '{"pickupLocation":{"lat":21.0285,"long":105.8542},"destination":{"lat":21.035,"long":105.821}}'

# 2. Tạo ride → 201: ride_123, REQUESTED
curl -i http://localhost:8081/rides -H 'Content-Type: application/json' \
  -d '{"fareId":"fare_123"}'

# 3. Đọc ride → 200: echo rideId, REQUESTED
curl -i http://localhost:8081/rides/ride_123

# 4. Phản hồi → 200: ACCEPTED (ACCEPT) hoặc REQUESTED (DENY)
curl -i -X PATCH http://localhost:8081/rides/ride_123 \
  -H 'Content-Type: application/json' -d '{"action":"ACCEPT"}'

# 5. Cập nhật trạng thái → 200: echo status
curl -i -X PATCH http://localhost:8081/rides/ride_123/status \
  -H 'Content-Type: application/json' -d '{"status":"COMPLETED"}'
```

### Location (8082)

```bash
# 6. Tọa độ → 200: driver_456 và tọa độ đã gửi
curl -i http://localhost:8082/drivers/location -H 'Content-Type: application/json' \
  -d '{"lat":0,"long":0}'

# 7. Availability → 200: false là giá trị hợp lệ
curl -i -X PATCH http://localhost:8082/drivers/availability \
  -H 'Content-Type: application/json' -d '{"available":false}'

# 8. Nearby → 200: danh sách một driver fixture, distanceMeters=500
curl -i 'http://localhost:8082/internal/drivers/nearby?lat=21.0285&long=105.8542&radiusMeters=3000'
```

### Matching (8083)

```bash
# 9. Yêu cầu matching → 202: echo rideId, MATCHING
curl -i http://localhost:8083/internal/matching/requests \
  -H 'Content-Type: application/json' -d '{"rideId":"ride_123"}'

# 10. Đọc matching → 200: echo rideId, MATCHING
curl -i http://localhost:8083/internal/matching/requests/ride_123
```

### Notification (8084)

```bash
# 11. Yêu cầu thông báo → 202: notification_123, QUEUED và các trường đã gửi
curl -i http://localhost:8084/internal/notifications/ride-requests \
  -H 'Content-Type: application/json' \
  -d '{"rideId":"ride_123","driverId":"driver_456","type":"RIDE_REQUEST"}'
```

### Health, validation và lỗi

Cả bốn service có `GET /healthz`, ví dụ:

```bash
curl -i http://localhost:8081/healthz -H 'X-Request-ID: demo-request'
```

Response `{"service":"ride-service","status":"ok"}`. Header `X-Request-ID` được echo nếu có (tối đa 128 ký tự), tự sinh nếu thiếu hoặc dài hơn giới hạn; access log JSON chứa cùng request ID. Server tắt graceful khi nhận SIGINT/SIGTERM, đợi tối đa 10 giây.

- Body phải là một JSON document, tối đa 1 MiB; input lỗi trả 400.
- `lat` ∈ [-90, 90], `long` ∈ [-180, 180]; số 0 hợp lệ, thiếu trường không hợp lệ.
- `available` bắt buộc xuất hiện, chấp nhận true và false.
- ID là chuỗi khác rỗng/không chỉ chứa whitespace; không ép UUID hoặc kiểm tra tồn tại.
- `action`: `ACCEPT` / `DENY`; status cập nhật: `ARRIVED` / `IN_PROGRESS` / `COMPLETED`.
- Nearby yêu cầu query `lat`, `long`; `radiusMeters` > 0, mặc định 3000.
- Notification yêu cầu `rideId`, `driverId`, `type: RIDE_REQUEST`.
- Route không tồn tại trả 404, method sai trả 405, lỗi server trả 500.

Mọi lỗi dùng envelope:

```json
{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body","details":["..."]}}
```

`details` là mảng chuỗi, có thể rỗng; thông báo validation chỉ mang tính mô tả, không phải mã lỗi ổn định để client phân nhánh.

## Giới hạn của mock

- Middleware gắn `rider_123` hoặc `driver_456` theo route, không cần token. `driverId` trong notification body là người nhận thông báo.
- Không lưu dữ liệu hoặc kiểm tra chuyển trạng thái. **GET sau PATCH vẫn trả fixture `REQUESTED`**; GET matching luôn `MATCHING` cho mọi ID hợp lệ.
- Fare/ride/notification ID tạo mới luôn là ID mẫu. Nearby luôn trả cùng driver, không lọc theo tọa độ, bán kính hoặc availability.
- Các request `202` chỉ nhận acknowledgement mock; không có worker, queue, matching hoặc push thật. Tạo ride không gọi Matching hay Notification.
- API `/internal/...` chưa có xác thực, chỉ phục vụ scaffold local. Không có kiểm tra business như fareId/rideId có tồn tại hay không.

## Kiểm tra

```bash
make test
make vet
make build
```

Test dùng `testing`/`httptest`, kiểm tra đủ 11 endpoint, JSON response, validation, enum, tọa độ biên/zero, boolean false, statelessness, health, request ID, recovery và lỗi 404/405/500. Binary được tạo trong `bin/`.

OpenAPI 3.0.3 trong `docs/openapi/` có thể import trực tiếp vào Swagger Editor hoặc công cụ API client, không cần chạy codegen.

Sau khi bốn service đang chạy (Go hoặc Compose), smoke test health và đủ 11 endpoint:

```bash
python3 scripts/smoke.py
```

Script chỉ dùng thư viện chuẩn Python 3. Có thể đổi địa chỉ qua `RIDE_URL`, `LOCATION_URL`, `MATCHING_URL`, `NOTIFICATION_URL`, ví dụ `RIDE_URL=http://localhost:9081 python3 scripts/smoke.py`.
# int3105-ride-share
