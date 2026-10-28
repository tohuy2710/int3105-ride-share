# Phát triển nhiều service

## Trạng thái service

Ride lưu trạng thái trong PostgreSQL, Redis hỗ trợ contention gate và Auth dùng fixture/JWT. Location, Matching và Notification có application/repository PostgreSQL riêng; entrypoint HTTP hiện tại vẫn dùng `MockService`. RabbitMQ chỉ có topology, chưa có publisher/consumer hoặc lời gọi HTTP liên service.

Mỗi thay đổi nên nằm trên branch hoặc worktree riêng. Giữ nguyên test bảo vệ ACCEPT, ownership, retry, cancellation/status và cả hai mode `sql`/`redis_sql`. Không ghi đè thay đổi chưa commit của người khác.

## Ranh giới kiến trúc

- `router.go` gắn middleware; `handler.go` bind/validate và map lỗi; không chứa SQL/Redis.
- `service.go` chỉ nghiệp vụ và interface; không import Gin hoặc thư viện DB/broker.
- DTO input có binding nằm ở lớp HTTP; service nhận giá trị thuần, dùng `contracts.Coordinates` thay vì con trỏ validation.
- Repository là interface ở phía nghiệp vụ; implementation SQL nằm trong adapter. Không dùng DB của service khác.
- `internal/contracts` không import framework; chỉ thay đổi có chủ ý cùng OpenAPI/event schema.
- `internal/clients` chứa interface và stub. Chưa khởi tạo HTTP client hay publisher trong application. `*_URL` và `RABBITMQ_URL` dành cho bước nối transport sau.

## Contract trước implementation

OpenAPI 3.0.3 nằm trong `docs/openapi`. API catalog mô tả fixture và caller dự kiến. Giữ camelCase, enum, nullability, status code, error envelope và ý nghĩa mock đúng contract.

Khi thay đổi API, cập nhật OpenAPI/catalog và thống nhất consumer trước, sau đó cập nhật `internal/contracts`, DTO/client, handler mapping và test response thực tế đồng bộ. Giữ test fixture mock hiện tại và thêm test riêng cho runtime thật. Không đổi fixture hoặc expected để che regression và không thay luồng Ride đang dùng PostgreSQL bằng mock.

## Chạy và kiểm tra

```bash
cp .env.example .env
make infra-up
make migrate
make run-auth
# Terminal khác: make run-ride / run-location / run-matching / run-notification
make fmt-check vet test-race contract build
```

Go chạy trực tiếp không tự load `.env`. Để chạy thêm replica: `PORT=9081 make run-ride`; cùng `DATABASE_URL`, `REDIS_URL`, JWT config và `ACCEPT_MODE` với replica đầu.

Integration với Compose infra:

```bash
TEST_DATABASE_URL='postgres://ride:ride_local@localhost:5432/rides?sslmode=disable' \
TEST_REDIS_URL='redis://localhost:6379/0' make integration
```

Hoặc `python3 scripts/native_check.py` tạo database/Redis riêng. Native runner cần `postgres`, `initdb`, `pg_ctl`, `createdb` cùng phiên bản, `redis-server`, `redis-cli`, compiler cho Go race detector; dùng bindir của `postgres` để tránh lẫn phiên bản. Không xóa dữ liệu dependency khác. Mỗi run giữ log/data trong `.cache`; tiến trình được dừng khi hoàn tất hoặc có exception.

Native runner còn tạo `location_test`, `notification_test`, `matching_test` trong PostgreSQL tạm và truyền từng `TEST_<SERVICE>_DATABASE_URL` để chạy test repository/application với race detector. SQL service chỉ được áp dụng rõ ràng bởi setup test; default runtime vẫn mock. Runner kiểm tra Swagger UI/spec/assets trên cả năm service cùng HTTP smoke.

Mỗi service phục vụ Swagger UI tại `/swagger/index.html`, JSON tại `/swagger/openapi.json`, YAML gốc tại `/swagger/openapi.yaml`. Assets được nhúng trong binary; sửa OpenAPI rồi build lại để cập nhật tài liệu. JSON dùng server `/` để **Try it out** hoạt động với custom `PORT`; JWT/internal token vẫn bắt buộc cho các API tương ứng.

## Migration và cấu hình

Ứng dụng không tự chạy migration. Bootstrap hiện tại chạy transaction và advisory lock khi gọi `cmd/migrate`; có thể chạy lại. Từ migration tiếp theo, cần thêm migration mới và cơ chế version/checksum trước khi thay đổi schema; không sửa bootstrap cho database đã triển khai. SQL của các repository thật nằm trong `internal/<service>/postgres/migrations/001_init.sql`, chỉ tác động schema riêng `location`, `notification` hoặc `matching`. Các migration này được kiểm thử độc lập và chưa nằm trong runtime/runner chung. Không query hoặc tạo foreign key xuyên service.

`JWT_SECRET` tối thiểu 32 byte, `INTERNAL_TOKEN` tối thiểu 16 byte; thiếu cấu hình làm service fail startup. RIDER/DRIVER lấy từ JWT, không tin `X-Driver-ID` hoặc ID trong body. Internal token dùng riêng cho service API. DB pool mặc định 10 connection/replica; timeout DB 2 giây; Redis timeout 100 ms; lock TTL 5 giây.

## Hoàn tất PR

Mô tả hành vi cuối cùng, contract thay đổi, kiểm tra đã chạy và giới hạn. CI phải qua format, vet, unit/race, contract, build và integration. Không commit `.env`, cache, binary, log, data directory hoặc benchmark kết quả giả làm số liệu Kaggle.

## Bật implementation thật

- Location: nối application/repository vào handler, map validation/dependency errors, kiểm tra DB readiness và thêm test nearby với dữ liệu động.
- Notification: công bố ID động và idempotency theo `(rideId, driverId, type)`, thêm GET internal theo ID và lỗi `NOTIFICATION_NOT_FOUND`. Chỉ lưu `QUEUED`; chưa có provider hoặc delivery worker.
- Matching: công bố pickup bắt buộc, radius mặc định 3000, trạng thái `MATCHING`/`NO_DRIVERS`/`MATCHED`/`CANCELLED` và các lỗi tương ứng. Nối các port RideReader, DriverFinder và NotificationRequester qua transport thật; không inject fake/stub vào runtime.

Trước khi bật, đưa SQL đã review vào migration runner có version/checksum và áp dụng bằng command riêng. Không auto-migrate từ replica. Không suy ra pickup bằng tọa độ giả, không để Matching gán driver hoặc gọi ACCEPT, không thêm remote I/O đồng bộ vào transaction ACCEPT.

Sau khi đổi runtime, chạy PostgreSQL integration, contract/security/error tests, baseline race/fault tests, HTTP smoke và benchmark ngắn hai replica ở cả hai acceptance mode. Chỉ mô tả endpoint hoặc transport đã bật khi có bằng chứng chạy thật.
