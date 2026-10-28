# Ride Share — Pha 1

Backend Go/Gin gồm 5 service độc lập. Ride lưu trạng thái thật trong PostgreSQL; Redis giảm contention khi nhiều tài xế nhận cùng cuốc. Auth kiểm tra tài khoản fixture và phát JWT. Các nghiệp vụ còn lại trả mock theo OpenAPI.

| Service | Port | Trách nhiệm | Contract |
| --- | --- | --- | --- |
| Ride | 8081 | Tạo/đọc/nhận/hủy/chuyển trạng thái thật; giá mock | [ride.yaml](docs/openapi/ride.yaml) |
| Location | 8082 | Mock tọa độ, availability, nearby | [location.yaml](docs/openapi/location.yaml) |
| Matching | 8083 | Mock acknowledgement và trạng thái MATCHING | [matching.yaml](docs/openapi/matching.yaml) |
| Notification | 8084 | Mock acknowledgement QUEUED | [notification.yaml](docs/openapi/notification.yaml) |
| Auth | 8085 | Login fixture, JWT, register mock | [auth.yaml](docs/openapi/auth.yaml) |

HTTP client stub có kiểu dữ liệu thay cho OpenFeign trong Go. RabbitMQ chạy thật với topology khai báo, **chưa có publisher/consumer hoặc lời gọi HTTP liên service**. Tạo ride chưa kích hoạt matching; GET Matching vẫn `MATCHING` sau khi Ride được nhận.

Location, Notification và Matching đã có application và PostgreSQL repository riêng. Các entrypoint vẫn dùng mock theo bảng trên; SQL service chưa nằm trong migration runner chung và endpoint thật chưa được bật.

## Swagger UI

Sau khi chạy service, mở `/swagger/index.html` trên port tương ứng:
[Ride](http://localhost:8081/swagger/index.html),
[Location](http://localhost:8082/swagger/index.html),
[Matching](http://localhost:8083/swagger/index.html),
[Notification](http://localhost:8084/swagger/index.html),
[Auth](http://localhost:8085/swagger/index.html).

UI và OpenAPI được nhúng trong binary, không cần CDN. JSON ở `/swagger/openapi.json` dùng cùng host/port với service cho **Try it out**, kể cả khi đổi `PORT`; YAML gốc ở `/swagger/openapi.yaml`. Tài liệu thể hiện runtime hiện tại, gồm các API mock. Dùng **Authorize** để nhập JWT từ `/auth/login` hoặc `X-Internal-Token` đúng loại API; quyền truy cập endpoint vẫn được kiểm tra như bình thường.

## Chạy nhanh bằng Docker

Yêu cầu Docker Engine/Desktop và Compose v2 hỗ trợ `--wait`.

```bash
cp .env.example .env
make compose-up
make smoke
make compose-down
```

Compose chạy 5 ứng dụng, PostgreSQL, Redis, RabbitMQ Management và migration job riêng. Các port chỉ bind localhost. Migration hoàn tất trước khi Ride khởi động; ứng dụng không tự migrate. Dữ liệu PostgreSQL và RabbitMQ được giữ trong named volume; `compose-down` không xóa volume.

RabbitMQ UI: http://localhost:15672, tài khoản local `ride` / `ride_local`. Các giá trị trong `.env.example` phục vụ môi trường phát triển.

## Chạy từng service để phát triển

Toolchain chuẩn là Go **1.25.9**; Go module yêu cầu ít nhất 1.25.0. Dependency nằm trong `go.mod/go.sum`, image và source native được khóa tại [infra/versions.json](infra/versions.json).

```bash
cp .env.example .env
make infra-up
make migrate
make run-auth
# Các terminal riêng:
make run-ride
make run-location
make run-matching
make run-notification
```

Make tự đọc/export `.env`; `go run` trực tiếp không đọc file này. Fixture 1 rider + 100 driver đã có sẵn, không cần seed lại. `make seed` tái tạo fixture, hash mật khẩu thay đổi nên chỉ chạy khi chủ ý cập nhật fixture. Custom fixture: `go run ./cmd/seed -drivers 200 -out .cache/accounts.json`, sau đó đổi `AUTH_FIXTURES` và khởi động lại Auth.

Mật khẩu fixture mặc định: `RideShare123!`.

| Email | ID | Role |
| --- | --- | --- |
| `rider@example.com` | `rider_123` | RIDER |
| `driver1@example.com` | `driver_456` | DRIVER |
| `driver2@example.com` | `driver_457` | DRIVER |
| `driver100@example.com` | `driver_555` | DRIVER |

Đăng nhập:

```bash
curl -s http://localhost:8085/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"rider@example.com","password":"RideShare123!"}'
```

Lấy `accessToken` từ response, dùng `Authorization: Bearer <token>` cho API public. `/auth/register` trả `persisted:false`, không tạo user, không phát token; email đăng ký mới không đăng nhập được.

```bash
curl http://localhost:8081/rides -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $RIDER_TOKEN" -d '{"fareId":"fare_123"}'

curl http://localhost:8081/rides/ACTUAL_RIDE_ID -H "Authorization: Bearer $RIDER_TOKEN"

curl -X PATCH http://localhost:8081/rides/ACTUAL_RIDE_ID \
  -H "Authorization: Bearer $DRIVER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"action":"ACCEPT"}'

# Hủy mềm cuốc chưa được nhận; retry hủy trả 200.
curl -X DELETE http://localhost:8081/rides/ANOTHER_REQUESTED_RIDE_ID \
  -H "Authorization: Bearer $RIDER_TOKEN"
```

## Kiến trúc và tính đúng đắn

`HTTP handler → application/service → repository hoặc client port → adapter`. Handler chuyển DTO có validation sang kiểu nghiệp vụ; `service.go` không import Gin, SQL driver, Redis hoặc RabbitMQ. SQL/data mapper nằm trong `internal/ride/postgres`; `internal/ride/runtime` là nơi nối dependency.

PostgreSQL chỉ có schema `ride`, bảng `rides` với ID, fare/rider/driver ID, status và timestamps. Không có user/fare/location table, foreign key xuyên service, CouchDB hoặc outbox. Auth fixture nằm trong file có hash bcrypt.

Hai bước nhận cuốc:

1. **Reduce contention:** `SET ride:accept:{rideId} <token> NX PX 5000`. Không lấy được gate trả `409 ACCEPT_IN_PROGRESS`, không vào DB. Nhả gate bằng Lua so token; Redis lỗi thì fallback SQL.
2. **Data transaction:** conditional `UPDATE ... WHERE status='REQUESTED' AND driver_id IS NULL ... RETURNING` trong transaction `READ COMMITTED`. Chỉ trả thành công sau commit. SQL là nơi quyết định cuối cùng kể cả khi gate hết TTL.

`ACCEPT_MODE=sql` bỏ gate; `redis_sql` là mặc định. Retry của người thắng khi cuốc còn `ACCEPTED` trả cùng kết quả; tài xế khác trả `409 RIDE_ALREADY_ACCEPTED`. Một tài xế vẫn có thể nhận nhiều cuốc khác nhau trong baseline này.

Chuyển trạng thái: `REQUESTED → ACCEPTED → ARRIVED → IN_PROGRESS → COMPLETED`; hủy chỉ từ `REQUESTED → CANCELLED`. GET đọc trạng thái đã lưu. `driverId` luôn có trường, nhận `null` khi chưa có tài xế. `DENY` chỉ đọc, không lưu lượt từ chối.

## Kiểm tra

```bash
make fmt-check vet test-race contract build
# Nếu có PostgreSQL/Redis native trên PATH, tự dựng instance riêng và dọn tiến trình:
python3 scripts/native_check.py
# Hoặc dùng dependency đang chạy, xem docs/development.md:
make integration
```

Integration kiểm tra hai replica, duy nhất một người thắng, retry, ACCEPT/DELETE cạnh tranh, ownership/status, Redis lỗi, gate bị giữ, TTL, tiến trình giữ gate thoát và DB lỗi. Smoke kiểm tra API thật cùng contract các service mock.

`GET /healthz` là liveness. `/readyz` của Ride yêu cầu bảng PostgreSQL; Redis lỗi trả HTTP 200 với `status:degraded`. Các service khác kiểm tra được cấu hình lúc khởi động và không phụ thuộc broker.

## Benchmark và Kaggle

```bash
make build
python3 -m venv .cache/benchmark-venv
.cache/benchmark-venv/bin/python -m pip install -r scripts/benchmark-requirements.txt
make benchmark-suite PYTHON=.cache/benchmark-venv/bin/python
# Kiểm tra nhanh load generator, không dùng kết quả làm đánh giá hiệu năng:
python3 scripts/benchmark_suite.py --concurrency 2,10 --groups 5 --warmup 1 --repeats 2 --no-plots
```

Suite cần PostgreSQL/Redis native trên PATH, khởi động instance riêng và hai replica cho từng chế độ; chưa hỗ trợ dùng lại DB production. Kết quả mặc định ở `.cache/benchmark/run-*` gồm JSON/CSV, CPU/RAM, log, metadata và biểu đồ. Xem [protocol benchmark](docs/benchmark.md).

Notebook: [kaggle_cpu.ipynb](notebooks/kaggle_cpu.ipynb). Hướng dẫn [Kaggle](docs/kaggle.md) có bootstrap native đã khóa source/checksum, không cần Docker daemon. Có thể đóng gói chính source local bằng `python3 scripts/package_kaggle.py` để attach Dataset. Chưa coi kiểm chứng Kaggle hoàn tất cho đến khi có `Save & Run All` thành công trên tài khoản Kaggle.

## Tài liệu phát triển

- [API catalog và giao tiếp](docs/api-catalog.md): output, security, errors và stub.
- [Phát triển nhiều service](docs/development.md): kiến trúc, migration, kiểm thử và quy trình contract.
- [Event contract](docs/events.md): topology RabbitMQ, schema và yêu cầu khi nối transport.
- [Đối chiếu đề Pha 1](docs/phase1.md): tiêu chí và phần còn cần xác minh.
