# Phát triển nhiều service

## Trạng thái service

Ride lưu trạng thái trong PostgreSQL, Redis hỗ trợ contention gate và Auth dùng fixture/JWT. Location, Matching và Notification đã dùng application/repository PostgreSQL thật. Ride ghi outbox rồi gọi Matching; Matching gọi Ride/Location/Notification bằng HTTP có internal token. RabbitMQ chỉ có topology, chưa có publisher/consumer.

Mỗi thay đổi nên nằm trên branch hoặc worktree riêng. Giữ nguyên test bảo vệ ACCEPT, ownership, retry, cancellation/status và cả hai mode `sql`/`redis_sql`. Không ghi đè thay đổi chưa commit của người khác.

## Ranh giới kiến trúc

- `router.go` gắn middleware; `handler.go` bind/validate và map lỗi; không chứa SQL/Redis.
- `service.go` chỉ nghiệp vụ và interface; không import Gin hoặc thư viện DB/broker.
- DTO input có binding nằm ở lớp HTTP; service nhận giá trị thuần, dùng `contracts.Coordinates` thay vì con trỏ validation.
- Repository là interface ở phía nghiệp vụ; implementation SQL nằm trong adapter. Không dùng DB của service khác.
- `internal/contracts` không import framework; chỉ thay đổi có chủ ý cùng OpenAPI/event schema.
- `internal/clients` chứa interface, HTTP adapter có timeout và stub cho test độc lập. Runtime dùng `*_URL` và `INTERNAL_TOKEN`; business application nhận các port. Không theo redirect với internal credentials.

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

Native runner còn tạo `location_test`, `notification_test`, `matching_test` trong PostgreSQL tạm và truyền từng `TEST_<SERVICE>_DATABASE_URL` để chạy test repository/application với race detector. SQL service được áp dụng bởi setup test hoặc migration command rõ ràng. Runner kiểm tra Swagger UI/spec/assets trên cả năm binary cùng HTTP smoke, gồm matching thật và notification dedup.

Mỗi service phục vụ Swagger UI tại `/swagger/index.html`, JSON tại `/swagger/openapi.json`, YAML gốc tại `/swagger/openapi.yaml`. Assets được nhúng trong binary; sửa OpenAPI rồi build lại để cập nhật tài liệu. JSON dùng server `/` để **Try it out** hoạt động với custom `PORT`; JWT/internal token vẫn bắt buộc cho các API tương ứng.

## Migration và cấu hình

Ứng dụng không tự chạy migration. Runner chạy transaction và advisory lock khi gọi `cmd/migrate`; lưu version/checksum tại `platform.schema_migrations`, phát hiện migration đã áp dụng bị sửa. `001_rides.sql` được giữ nguyên; `002_matching_flow.sql` thêm schema service và outbox. Có thể nâng DB chỉ có bootstrap cũ hoặc chạy lại mà không mất dữ liệu. Chỉ thêm migration tiếp theo, không sửa SQL đã triển khai. SQL service trong `internal/<service>/postgres/migrations` tiếp tục phục vụ integration test độc lập; production schema do root migration versioned sở hữu. Không query hoặc tạo foreign key xuyên service.

`JWT_SECRET` tối thiểu 32 byte, `INTERNAL_TOKEN` tối thiểu 16 byte; thiếu cấu hình làm service fail startup. RIDER/DRIVER lấy từ JWT, không tin `X-Driver-ID` hoặc ID trong body. Internal token dùng riêng cho service API. DB pool mặc định 10 connection/replica; timeout DB 2 giây; Redis timeout 100 ms; lock TTL 5 giây.

## Hoàn tất PR

Mô tả hành vi cuối cùng, contract thay đổi, kiểm tra đã chạy và giới hạn. CI phải qua format, vet, unit/race, contract, build và integration. Không commit `.env`, cache, binary, log, data directory hoặc benchmark kết quả giả làm số liệu Kaggle.

## Luồng đã tích hợp

Location/Matching/Notification chạy implementation thật. POST tạo cuốc cần pickup rõ ràng; không suy ra từ fare mock hoặc tự chèn tọa độ. Ride lưu outbox trong cùng transaction, dispatcher claim qua lease 15 giây và retry sau restart. Không thêm remote I/O vào transaction ACCEPT.

Migration phải chạy trước khi deploy binary mới; Compose bảo đảm dependency này. Cần chạy lại `make migrate` khi nâng bản local host. Giữ `RIDE_URL`, `LOCATION_URL`, `MATCHING_URL`, `NOTIFICATION_URL` đúng địa chỉ peer (Compose dùng hostname nội bộ; native dùng localhost + custom port). Trong test, thiết lập đủ URL trước khi tạo child process.

Integration kiểm tra HTTP và PostgreSQL thật, mất acknowledgement, restart, dedup, NO_DRIVERS, MATCHED/CANCELLED, readiness lỗi DB, rollback khi outbox lỗi và lease nhiều replica. Baseline race/fault và benchmark ACCEPT tiếp tục bảo vệ SQL/Redis; core-only benchmark không đo matching hoặc push throughput. Xem giới hạn trong API catalog.
