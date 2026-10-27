# Phát triển nhiều service

## Ownership và phân công hiện tại

Theo [bộ phân công cho 3 người](team/README.md):

| Người | Phần được giao | Instruction |
| --- | --- | --- |
| Người 1 | Application/repository PostgreSQL thật, SQL/test riêng Location | [Prompt Location](team/location-agent.md) |
| Người 2 | Application/repository PostgreSQL thật, SQL/test riêng Matching; fake remote ports trong test | [Prompt Matching](team/matching-agent.md) |
| Người 3 | Application/repository PostgreSQL thật, SQL/test riêng Notification, chưa delivery/push | [Prompt Notification](team/notification-agent.md) |
| Máy hiện tại — người tích hợp | Ride/race, Auth và toàn bộ phần hiện có dùng chung, contract/migration/runtime/HTTP/RabbitMQ, benchmark/Kaggle, tích hợp cuối | [Checklist tích hợp](team/integration.md) |

Ba người làm trong `internal/<service>/` và file handoff của mình, giữ nguyên mock/interface/handler/router/DTO đang chạy. `cmd/`, platform, contracts/clients, OpenAPI, shared tests, dependencies, Compose/CI, root migration và benchmark do người tích hợp giữ. Không còn phân Auth hoặc Ride cho ba người này. Phạm vi chi tiết trong [team README](team/README.md) được ưu tiên cho đợt giao việc này.

Mỗi dev dùng branch/worktree riêng từ commit tài liệu chung được người tích hợp thông báo; ghi BASE_SHA thực tế. Baseline race được bảo vệ là `8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9`, không phải yêu cầu quay về commit cũ làm mất instruction mới. Ví dụ khi đã có COMMON_BASE_SHA đúng:

```bash
git worktree add ../ride-share-location -b feat/location-logic "$COMMON_BASE_SHA"
```

Không ghi đè thay đổi người khác, không tự push main/merge. Đề xuất thay đổi shared contract/dependency/config trong [handoff](team/handoff-template.md); tiếp tục phần application/repository độc lập bằng interface/model nội bộ khi cần. Người tích hợp công bố contract đích rồi nối runtime sau khi nhận code. Không tự sửa scope hoặc fixture để làm test qua.

## Ranh giới kiến trúc

- `router.go` gắn middleware; `handler.go` bind/validate và map lỗi; không chứa SQL/Redis.
- `service.go` chỉ nghiệp vụ và interface; không import Gin hoặc thư viện DB/broker.
- DTO input có binding nằm ở lớp HTTP; service nhận giá trị thuần, dùng `contracts.Coordinates` thay vì con trỏ validation.
- Repository là interface ở phía nghiệp vụ; implementation SQL nằm trong adapter. Không dùng DB của service khác.
- `internal/contracts` không import framework; chỉ thay đổi có chủ ý cùng OpenAPI/event schema.
- `internal/clients` chứa interface và stub. Chưa khởi tạo HTTP client hay publisher trong application. `*_URL` và `RABBITMQ_URL` dành cho bước nối transport sau.

## Contract trước implementation

OpenAPI 3.0.3 nằm trong `docs/openapi`. API catalog mô tả fixture và caller dự kiến. Giữ camelCase, enum, nullability, status code, error envelope và ý nghĩa mock đúng contract.

Người tích hợp thay đổi API theo thứ tự: công bố OpenAPI/catalog và thống nhất consumer, cập nhật `internal/contracts`/DTO/client stub, handler mapping và test response thực tế. Dev service mô tả delta trong handoff, chưa sửa contract chung hoặc endpoint đang mock. Không đổi fixture hoặc response vì tiện triển khai; thay đổi phá vỡ tương thích cần người tích hợp duyệt. Không tự mock luồng Ride đang dùng PostgreSQL để làm test qua.

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

Ứng dụng không tự chạy migration. Bootstrap hiện tại chạy transaction và advisory lock khi gọi `cmd/migrate`; có thể chạy lại. Từ migration tiếp theo, cần thêm migration mới và cơ chế version/checksum trước khi thay đổi schema; không sửa bootstrap cho database đã triển khai. Không thêm bảng cho API còn mock trừ phần repository thật được giao rõ ràng trong team requirements: SQL riêng schema service được bàn giao/test độc lập, chưa được đưa vào runtime/runner chung.

`JWT_SECRET` tối thiểu 32 byte, `INTERNAL_TOKEN` tối thiểu 16 byte; thiếu cấu hình làm service fail startup. RIDER/DRIVER lấy từ JWT, không tin `X-Driver-ID` hoặc ID trong body. Internal token dùng riêng cho service API. DB pool mặc định 10 connection/replica; timeout DB 2 giây; Redis timeout 100 ms; lock TTL 5 giây.

## Hoàn tất PR

Mô tả hành vi cuối cùng, contract thay đổi, kiểm tra đã chạy và giới hạn. CI phải qua format, vet, unit/race, contract, build và integration. Không commit `.env`, cache, binary, log, data directory hoặc benchmark kết quả giả làm số liệu Kaggle.
