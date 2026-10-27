# Bàn giao Notification — Người 3

Áp dụng [mẫu bàn giao](../handoff-template.md), [yêu cầu Notification](../notification-requirements.md) và [prompt Notification](../notification-agent.md).

## Trạng thái và Git

- Service / người phụ trách: Notification / Người 3 (KhoiTran05).
- Trạng thái: **IN_PROGRESS**. Code và test đã xong trên working tree; chuyển sang READY_FOR_REVIEW sau khi commit/push theo yêu cầu của chủ task.
- BASE_SHA: `06142b4aa7b49adc2e0ec991a60bcfeaee142fcc` (`docs: assign service development and protect race baseline`, chứa bộ instruction; baseline race `8dd4bd0` là tổ tiên của commit này).
- HEAD_SHA: chưa commit. Thay đổi đang nằm trên working tree của branch `feature/khoitlm`. SHA sẽ được ghi sau khi commit, không đoán trước.
- Branch / PR URL: `feature/khoitlm` (tên đề xuất trong tài liệu là `feat/notification-logic`). Chưa push, chưa mở PR.
- Chỉ thay đổi trong ownership: **có**. `git diff --name-only 06142b4` cộng với file untracked chỉ gồm:
  - `internal/notification/application.go`
  - `internal/notification/application_test.go`
  - `internal/notification/postgres/repository.go`
  - `internal/notification/postgres/repository_integration_test.go`
  - `internal/notification/postgres/migrations/001_init.sql`
  - `internal/notification/postgres/migrations/migrations.go`
  - `docs/team/handoffs/notification.md` (file này)

## Đã triển khai và chưa triển khai

- Nghiệp vụ đã hoàn thành:
  - Enqueue `RIDE_REQUEST`: validate ID không rỗng (theo `strings.TrimSpace`) và type, sinh ID thật `notification_<32 hex>` (crypto/rand), lưu PostgreSQL. Chỉ trả `QUEUED` sau khi statement autocommit đã commit.
  - Idempotency theo `(rideId,driverId,type)`: cơ chế cuối cùng là UNIQUE constraint trong DB (`INSERT … ON CONFLICT ON CONSTRAINT … DO NOTHING RETURNING`, sau đó SELECT theo khóa nghiệp vụ). Không dùng map/mutex, và không SELECT trước INSERT. Retry tuần tự, request đồng thời qua hai pool/application, và trường hợp sau restart đều trả cùng ID; `created_at` và row không bị ghi lại.
  - Đọc theo ID (`GetNotification`) không gây side effect (đã kiểm `xmin`/`created_at` không đổi).
  - Lỗi DB, timeout hoặc commit không xác định đều trả `fault.Unavailable` (DEPENDENCY_UNAVAILABLE), không bao giờ trả acknowledgement giả.
- File/constructor đã thêm:
  - `notification.Notification{ID, RideID, DriverID, Type, Status}`: snapshot 5 trường, không có timestamp.
  - `notification.Repository` (port): `Enqueue(ctx, Notification) (Notification, error)`, `Get(ctx, id) (Notification, error)`.
  - `notification.NewApplication(repo) *Application`, các trường `Repo`, `DBTimeout` (mặc định 2s nếu ≤0) và `NewID` (nil thì dùng random).
  - Phương thức: `EnqueueRideRequest(ctx, rideID, driverID, type)`, `GetNotification(ctx, id)`, `SendRideRequest(ctx, contracts.NotificationRequest)` (cài đặt interface `Service` hiện có) và `Notification.Response()`.
  - Lỗi: `notification.ErrNotFound` (`NOTIFICATION_NOT_FOUND`), `notification.ErrValidation` (`VALIDATION_ERROR`), cả hai là `*fault.Error`; lỗi DB dùng `fault.Unavailable` có sẵn.
  - `postgres.NewRepository(*sql.DB) *Repository` (`internal/notification/postgres/repository.go`).
  - `migrations.Init` (string embed của `001_init.sql`, để runner của người tích hợp hoặc test áp dụng rõ ràng).
- Phần baseline còn nguyên: `service.go` (`Service`, `MockService` trả `notification_123`), `router.go`, `handler.go`, `dto.go`, `cmd/notification` và default runtime mock **không đổi**. `TestMockFixtureUnchanged` khóa fixture mock.
- Chưa làm / giới hạn (đúng phạm vi):
  - Không có endpoint GET, không có wiring `cmd/notification`, chưa đổi runtime mặc định.
  - Không có provider/push/email/SMS/websocket, worker, retry delivery, RabbitMQ consumer/publisher, HTTP call liên service và trạng thái SENT/DELIVERED.
  - Không xác minh ride/driver tồn tại (không FK/query sang Ride/Auth).
  - Handler hiện tại map mọi lỗi thành 500 `INTERNAL_ERROR`. Nếu nối `Application` vào router cũ mà chưa sửa handler, lỗi 503/400 sẽ hiển thị thành 500 (xem mục delta).

## Cách người tích hợp khởi tạo

Đối chiếu `internal/notification/application.go` và `internal/notification/postgres/repository.go`:

```go
import (
	"int3105-ride-share/internal/notification"
	notificationpg "int3105-ride-share/internal/notification/postgres"
	"int3105-ride-share/internal/platform/database"
)

db, err := database.Open() // DATABASE_URL trỏ tới DB chứa schema notification
if err != nil { /* fail startup */ }
app := notification.NewApplication(notificationpg.NewRepository(db))
app.DBTimeout = 2 * time.Second // tùy chọn, mặc định 2s
notification.RegisterRoutes(router, app) // POST hiện có dùng implementation thật
// GET mới: app.GetNotification(ctx, c.Param("notificationId")) -> n.Response()
```

Gói `postgres` import `notification` (model/lỗi). Wiring nên đặt ở package runtime riêng (tương tự `internal/ride/runtime`) để tránh vòng import.

## Database và cấu hình

- SQL: `internal/notification/postgres/migrations/001_init.sql`. File này tạo `CREATE SCHEMA IF NOT EXISTS notification` và bảng `notification.requests`:
  - `id text PRIMARY KEY CHECK (id ~ '\S')`
  - `ride_id`, `driver_id text NOT NULL CHECK (… ~ '\S')`
  - `type text NOT NULL CHECK (type IN ('RIDE_REQUEST'))`
  - `status text NOT NULL DEFAULT 'QUEUED' CHECK (status IN ('QUEUED'))`
  - `created_at timestamptz NOT NULL DEFAULT now()`
  - `CONSTRAINT requests_ride_driver_type_key UNIQUE (ride_id, driver_id, type)`. Repository tham chiếu tên constraint này; nếu đổi tên thì phải sửa cả hai.
  - Không có FK, outbox, delivery log, bảng users hay bảng broker.
- Áp dụng: chỉ một file. Integration test tự `ExecContext(migrations.Init)` vào DB test (idempotent nhờ `IF NOT EXISTS`). Runtime **không** tự migrate; người tích hợp đưa file vào runner có version/checksum.
- Tạo DB test riêng (một lần):
  `docker compose --profile infra exec -T postgres createdb -U ride notification_test`
- DSN env của integration test: `TEST_NOTIFICATION_DATABASE_URL`. Nếu thiếu DSN khi chạy tag `integration`, test `t.Fatal` (đã kiểm), không fallback RAM. Cleanup chỉ xóa row có `ride_id LIKE 'itest_<random>_%'`.
- Cấu hình runtime đề xuất: dùng chung `DATABASE_URL`/`DB_MAX_OPEN_CONNS` của `database.Open()` và `DB_TIMEOUT` (mặc định 2s). Không cần dependency mới (`database/sql` + `pgx/v5/stdlib` đã có trong go.mod).
- Không ảnh hưởng Ride: SQL và mọi query chỉ đụng `notification.requests`. Không đọc/ghi `ride.rides`, Redis key hay transaction ACCEPT.

## Delta contract cần máy tích hợp công bố

| Endpoint/port/model | Hiện tại | Đích | Tác động caller/test và mapping cần làm |
| --- | --- | --- | --- |
| POST `/internal/notifications/ride-requests` | Mock luôn trả `notificationId: "notification_123"`, lỗi 500 | Shape 202 giữ nguyên 5 trường. `notificationId` là ID thật `notification_<hex>`, ổn định cho cùng `(rideId,driverId,type)` (retry/đồng thời/restart) | Sửa handler: dùng `api.ServiceError` thay cho `Fail(500)` cố định. Thêm 503 `DEPENDENCY_UNAVAILABLE` vào OpenAPI. Caller/test không được giả định ID cố định khi chạy runtime thật; fixture mock `notification_123` vẫn đúng cho MockService |
| GET `/internal/notifications/{notificationId}` — **mới** | Không có | 200 cùng snapshot 5 trường; `api.Internal()` | Thêm route, handler, OpenAPI (`operationId` đề xuất `getNotification`) và client stub nếu có caller. Path ID rỗng/blank trả 400 `VALIDATION_ERROR` (application trả `ErrValidation`) |
| Mã lỗi `NOTIFICATION_NOT_FOUND` — **mới** | Không có trong enum | 404 | Thêm vào enum `ErrorResponse.code` và vào `api.ServiceError` (`case "NOTIFICATION_NOT_FOUND": 404`) |
| Mã lỗi `VALIDATION_ERROR` từ application | Chỉ phát sinh ở bind DTO | 400 | `api.ServiceError` hiện map code này thành 500; cần `case "VALIDATION_ERROR": 400` (hoặc giữ validate ở DTO và để path GET tự kiểm) |
| `DEPENDENCY_UNAVAILABLE` | Code có sẵn, chưa liệt kê cho endpoint notification | 503 cho cả POST/GET | `ServiceError` đã map 503; chỉ cần công bố trong OpenAPI |

Không thêm field, timestamp hay nullable field. `status` vẫn chỉ `QUEUED`, nghĩa là đã lưu DB, không có nghĩa là broker hay thiết bị đã nhận.

Ghi chú hành vi cho caller (Matching): khi gặp 503 do timeout, row **có thể** đã commit phía server (pgx hủy query bất đồng bộ). Retry cùng bộ ba sẽ an toàn và trả đúng một ID. `TestTimeoutWhileKeyLockedIsNotSuccess` đã quan sát và kiểm chứng điều này.

## Bằng chứng kiểm thử

Máy dev là Windows và không có Go/gcc native, nên mọi lệnh Go chạy trong container `golang:1.25.9` (linux/amd64, có gcc cho `-race`) nối mạng `ride-share_default` tới Postgres của Compose (`postgres:16.10-alpine`).

| Lệnh/kịch bản | Đã chạy? | Kết quả thực tế | Môi trường/giới hạn |
| --- | --- | --- | --- |
| `go test -race ./internal/notification/... -count=1` | Có | `ok internal/notification` (8 test: validation không gọi repo, ID thật + dedupe, lỗi repo/timeout không thành công giả, lỗi sinh ID, deadline 2s/cấu hình, adapter `Service`, fixture mock không đổi, kiểm import không có HTTP/AMQP/Gin/Redis/clients) | Container golang:1.25.9 |
| `go test -tags=integration ./internal/notification/postgres` **không có DSN** | Có | FAIL đúng như mong đợi: `set TEST_NOTIFICATION_DATABASE_URL …` | Xác nhận không fallback RAM |
| `TEST_NOTIFICATION_DATABASE_URL=postgres://ride:***@postgres:5432/notification_test go test -race -tags=integration ./internal/notification/... -count=5` | Có | `ok` cả 5 lần, gồm 10 test: persist + GET snapshot + GET không đổi `xmin`; validation/not found; CHECK schema từ chối type/status/ID rỗng; retry giữ ID và `created_at`; 32 goroutine qua 2 pool → 1 row/ID; khác ride/driver → khác ID; đóng pool rồi tạo mới → đọc và dedupe; pool đóng/host không tới được/context hủy → Unavailable, không có row; timeout khi khóa bị giữ → 503, retry hội tụ 1 row; chờ transaction xung đột (rollback → ID mới, commit → ID của transaction kia) | PostgreSQL 16 thật, DB `notification_test` |
| `make fmt-check vet test-race contract build` | Có | exit 0; `test-race` ok toàn repo (auth, clients, contracttest, notification, platform/api, platform/app, platform/security, ride); `contract` ok; build đủ 8 binary | Chạy trên bản sao chuẩn hóa LF trong container. Trên checkout Windows (`core.autocrlf=true`, working tree CRLF), `fmt-check` báo **mọi** file Go có sẵn ngoài phạm vi; đây là artefact line ending, không phải lỗi format thật, và CI Linux không gặp |
| `git diff --check` và kiểm tra phạm vi | Có | exit 0; không có trailing whitespace trong file mới; diff so với BASE chỉ gồm file trong `internal/notification/` + handoff | Không có tham chiếu `net/http`/`amqp`/`SENT`/`DELIVERED` trong code không phải test |

Chưa chạy: integration/fault/smoke/benchmark của baseline Ride (việc của người tích hợp; PR này không chạm Ride).

## Checklist trước khi gửi review

- [x] Code chỉ nằm trong service được giao và file handoff của mình.
- [x] Không đổi MockService/interface/router/handler/DTO hiện tại hoặc mặc định entrypoint.
- [x] Không sửa Ride/Auth/platform/contracts/clients, shared tests, root migration, môi trường, dependency hoặc benchmark.
- [x] SQL chỉ thuộc schema service; không có FK/query xuyên service, startup migration hoặc outbox.
- [x] PostgreSQL thật đã kiểm tra persistence/concurrency; các lỗi/giới hạn được ghi rõ.
- [x] Người tích hợp có đủ constructor, SQL, contract delta và lệnh test để tiếp nhận.
- [x] Không commit .env/cache/binary/log/JWT; không tự merge/push main. (`.env` local được tạo từ `.env.example` để chạy Compose và đã nằm trong `.gitignore`.)

## Dành cho người tích hợp

- Commit đã review/merge và thứ tự:
- Contract/migration/wiring đã áp dụng:
- Regression/race/fault/smoke/benchmark thực tế đã chạy:
- Mốc kết luận: application/repository accepted / endpoint real integrated / transport integrated.
- Phần còn lại:
