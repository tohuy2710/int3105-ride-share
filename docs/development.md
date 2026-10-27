# Phát triển nhiều service

## Ownership

| Vai trò | Phạm vi sửa chính |
| --- | --- |
| Auth dev | `cmd/auth`, `internal/auth` |
| Ride dev | `cmd/ride`, `internal/ride`, migration Ride |
| Location dev | `cmd/location`, `internal/location` |
| Matching dev | `cmd/matching`, `internal/matching` |
| Notification dev | `cmd/notification`, `internal/notification` |
| Người tích hợp | platform, contracts, clients, Compose, dependency, CI, benchmark, tài liệu chung |

Mỗi dev tạo branch/worktree riêng từ cùng commit đã khóa contract. Không chỉnh phần của dev khác để chữa lỗi tích hợp. Đề xuất contract trước, cập nhật fixture/client stub và test tương ứng rồi mới triển khai logic. Người tích hợp xử lý thay đổi `go.mod/go.sum`, Compose, platform và contract dùng chung để tránh xung đột. Đây là quy trình ownership, không phải cơ chế khóa Git; cherry-pick/merge qua PR và chạy CI.

Ví dụ tạo worktree từ thư mục repo:

```bash
git worktree add ../ride-share-location -b feat/location
```

## Ranh giới kiến trúc

- `router.go` gắn middleware; `handler.go` bind/validate và map lỗi; không chứa SQL/Redis.
- `service.go` chỉ nghiệp vụ và interface; không import Gin hoặc thư viện DB/broker.
- DTO input có binding nằm ở lớp HTTP; service nhận giá trị thuần, dùng `contracts.Coordinates` thay vì con trỏ validation.
- Repository là interface ở phía nghiệp vụ; implementation SQL nằm trong adapter. Không dùng DB của service khác.
- `internal/contracts` không import framework; chỉ thay đổi có chủ ý cùng OpenAPI/event schema.
- `internal/clients` chứa interface và stub. Chưa khởi tạo HTTP client hay publisher trong application. `*_URL` và `RABBITMQ_URL` dành cho bước nối transport sau.

## Contract trước implementation

OpenAPI 3.0.3 nằm trong `docs/openapi`. API catalog mô tả fixture và caller dự kiến. Giữ camelCase, enum, nullability, status code, error envelope và ý nghĩa mock đúng contract.

Khi thay đổi API: sửa OpenAPI và catalog, thống nhất với consumer, sửa `internal/contracts`/DTO/client stub, cập nhật test response thực tế. Không đổi fixture hoặc response vì tiện triển khai; thay đổi phá vỡ tương thích cần người tích hợp duyệt. Không tự mock luồng Ride đang dùng PostgreSQL để làm test qua.

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

## Migration và cấu hình

Ứng dụng không tự chạy migration. Bootstrap hiện tại chạy transaction và advisory lock khi gọi `cmd/migrate`; có thể chạy lại. Từ migration tiếp theo, cần thêm migration mới và cơ chế version/checksum trước khi thay đổi schema; không sửa bootstrap cho database đã triển khai. Không thêm bảng cho API còn mock.

`JWT_SECRET` tối thiểu 32 byte, `INTERNAL_TOKEN` tối thiểu 16 byte; thiếu cấu hình làm service fail startup. RIDER/DRIVER lấy từ JWT, không tin `X-Driver-ID` hoặc ID trong body. Internal token dùng riêng cho service API. DB pool mặc định 10 connection/replica; timeout DB 2 giây; Redis timeout 100 ms; lock TTL 5 giây.

## Hoàn tất PR

Mô tả hành vi cuối cùng, contract thay đổi, kiểm tra đã chạy và giới hạn. CI phải qua format, vet, unit/race, contract, build và integration. Không commit `.env`, cache, binary, log, data directory hoặc benchmark kết quả giả làm số liệu Kaggle.
