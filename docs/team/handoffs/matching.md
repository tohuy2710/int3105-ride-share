# Bàn giao Matching — Person 2

## Trạng thái và Git

- Service/người phụ trách: Matching / Person 2.
- Trạng thái: **IN_PROGRESS — unit/race/vet/contract/build đã PASS; PostgreSQL integration chưa xác minh, format-check baseline còn lỗi CRLF**.
- BASE_SHA: `06142b4aa7b49adc2e0ec991a60bcfeaee142fcc` (commit đang có đủ instruction chung).
- Protected race baseline: `8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9`.
- HEAD_SHA: **chưa commit**, HEAD vẫn là BASE_SHA; bàn giao working-tree changes, không tự đoán commit mới.
- Branch triển khai: `feat/matching-logic` trong checkout độc lập tại `C:/Users/admin/AppData/Local/Temp/ride-share-matching-work`. Chưa push/PR/merge.
- Workspace review hiện ở nhánh `feature/duc`, HEAD bằng BASE_SHA; không đổi nhánh workspace. Checkout triển khai riêng vẫn ở `feat/matching-logic`, cùng BASE_SHA. Đối chiếu SHA-256 ngày 2026-10-27: toàn bộ 7 file code/SQL mới giống nhau giữa hai checkout. Handoff được đồng bộ sau rà soát; chưa commit/push/merge.
- Chỉ thay đổi 8 file thuộc ownership:
  - `internal/matching/application.go`
  - `internal/matching/model.go`
  - `internal/matching/input_dto.go`
  - `internal/matching/application_test.go`
  - `internal/matching/application_integration_test.go`
  - `internal/matching/postgres/repository.go`
  - `internal/matching/postgres/migrations/001_init.sql`
  - `docs/team/handoffs/matching.md`

## Đã triển khai và giới hạn

- Application mới độc lập với `Service`/`MockService`: input có pickup thật do caller cung cấp, radius đã normalize; validate ID, giới hạn tọa độ, NaN/Inf và radius > 0.
- Helper boundary `InputFields.Normalize()` phân biệt thiếu tọa độ với số 0; radius thiếu mặc định 3000; radius null/không phải số/không hợp lệ bị từ chối. Không được suy ra pickup từ fare/Ride/fixture.
- Đọc Ride qua port chỉ đọc; job mới chỉ được tạo trên REQUESTED. Ride thiếu và dependency lỗi được phân biệt.
- DriverFinder chỉ được dùng khi chưa có job. Deduplicate ID, giữ nguyên thứ tự Location trả; input và danh sách ứng viên không thay đổi sau commit. NO_DRIVERS không tự re-match.
- Repository dùng PK ride_id và transaction READ COMMITTED: INSERT ON CONFLICT DO NOTHING, sau đó SELECT bằng statement riêng rồi commit. Tiến trình thua đọc job thắng; input khác trả conflict. Chỉ job đã commit được trả cho application để dispatch.
- Retry MATCHING gửi lại toàn bộ các bộ ba `(rideId,driverId,RIDE_REQUEST)` đã lưu. Chỉ trả success khi đã có đầy đủ acknowledgement QUEUED đúng ID/type. Lỗi một phần trả unavailable, giữ nguyên job. Notification phải deduplicate; không có exactly-once hoặc rollback side effect ở service khác.
- GET không tìm driver hoặc dispatch thông báo. GET/POST trên job hiện có phản ánh ACCEPTED/ARRIVED/IN_PROGRESS/COMPLETED thành MATCHED, hoặc CANCELLED thành CANCELLED.
- Cập nhật terminal có điều kiện theo status đang lưu. Terminal đầu tiên đã ghi được giữ nguyên; request/snapshot cũ không ghi lùi. Không thay input/candidates/createdAt khi retry hoặc đổi status.
- Không giữ transaction/connection DB trong các lời gọi Ride/Location/Notification. Context được truyền nguyên qua ports; lỗi context vẫn nhận diện bằng `errors.Is`, đồng thời là ErrUnavailable. Không trả raw SQL/DSN/dependency details.
- Có thể có hai nearby reads khi hai request cùng quan sát chưa có job; chỉ một tập ứng viên commit được dùng. Cả hai có thể gửi lại cùng tuple; Notification phải idempotent.
- Ride đổi sau snapshot vẫn có thể dẫn tới thông báo đến muộn, đúng giới hạn baseline. Matching không phải nguồn chuẩn về assignment.
- **Chưa bật endpoint thật**, chưa nối HTTP/AMQP, chưa worker, provider, outbox hoặc delivery. Fake remote ports chỉ có trong test, không fallback runtime.
- Mock, Service interface, DTO/router/handler cũ, cmd, shared contracts, OpenAPI và Ride/race đều giữ nguyên.

## Constructor, model và port

Tham chiếu [application](../../../internal/matching/application.go), [model/ports](../../../internal/matching/model.go), [boundary helper](../../../internal/matching/input_dto.go), [PostgreSQL adapter](../../../internal/matching/postgres/repository.go).

```go
// db là *sql.DB do runtime của integrator mở và quản lý; driver pgx đã đăng ký.
repo := postgres.NewRepository(db)
app := matching.NewApplication(repo, rideReader, driverFinder, notificationRequester)

// HTTP boundary tương lai decode JSON vào fields và xử lý lỗi decode trước.
in, err := fields.Normalize() // fields có kiểu matching.InputFields
if err != nil {
    // map sang 400 VALIDATION_ERROR
    return
}
reply, err := app.Request(ctx, in) // (matching.Result, error)
// GET tương lai: app.Get(ctx, rideID)
// reply chỉ chứa rideId/status; không trả Job nội bộ/candidates/timestamps.
```

| Port | Chữ ký / nghĩa |
| --- | --- |
| Constructor | `matching.NewApplication(MatchingRepository, RideReader, DriverFinder, NotificationRequester) *Application`; inject đủ dependency thật khi wiring |
| PostgreSQL constructor | `postgres.NewRepository(db *sql.DB) *Repository`; không migrate/start worker |
| RideReader | `GetRide(context.Context, string) (contracts.Ride, error)`; confirmed 404 phải map `matching.ErrRideNotFound`; lỗi khác là unavailable |
| DriverFinder | `Nearby(context.Context, contracts.NearbyInput) (contracts.NearbyResponse, error)`; lat/long/radius từ input thật; trả thứ tự Location |
| NotificationRequester | `RequestRide(context.Context, contracts.NotificationRequest) (contracts.NotificationResponse, error)`; phải acknowledge bền vững và deduplicate tuple |
| MatchingRepository | `Get(ctx, rideID)`, `CreateOrGet(ctx, Job)`, `AdvanceStatus(ctx, rideID, Status)`, đều trả `(Job,error)` |
| Application | `Request(ctx, matching.Input) (matching.Result,error)`, `Get(ctx,rideID) (matching.Result,error)` |

Ride/Location có signature tương ứng với shared client interfaces, nhưng chưa có transport thật. Notification port dùng `RequestRide`; shared client hiện dùng `Send`, cần adapter của integrator. Không inject Stub vào runtime thật. Application mới chủ ý không implement Service cũ vì input cũ không biểu diễn được pickup bắt buộc.

Unit tests dùng `memoryRepository`, `rideFunc`, `finderFunc`, `notifyFunc`, `repositoryOverride`. Integration tests chỉ fake ba remote ports, dùng adapter PostgreSQL thật, hai pool/application độc lập và DB riêng.

## Database và cấu hình

- SQL: [001_init.sql](../../../internal/matching/postgres/migrations/001_init.sql), chỉ schema `matching`, bảng `requests`.
- ride_id text PK; tọa độ hữu hạn và có giới hạn; radius hữu hạn > 0; status có enum CHECK; candidate_driver_ids JSONB array NOT NULL. MATCHING cần ứng viên; NO_DRIVERS cần mảng rỗng. Adapter còn từ chối candidate ID blank hoặc trùng.
- created_at/updated_at timestamptz mặc định now(). Không FK/query liên service, index ngoài PK, startup auto-migration, outbox, Redis hoặc bảng Ride.
- Migration này là đề xuất chưa nằm trong root runner. Integrator phải review và đưa vào migration version/checksum trước khi bật runtime; không sửa bootstrap đã triển khai.
- Integration setup đọc SQL rõ ràng trong TestMain, áp dụng khi bảng chưa có, dùng transaction/advisory lock chỉ cho setup test. Không drop/truncate schema; cleanup chỉ ID ngẫu nhiên do từng test tạo. Bảng đã tồn tại phải đúng migration, không âm thầm sửa shape.
- Chỉ chạy trên database test riêng (ví dụ tên `matching_test`), DSN qua `TEST_MATCHING_DATABASE_URL`; thiếu DSN dưới tag integration sẽ fail, không skip/RAM.
- Có thể áp dụng SQL thủ công một lần vào database test mới; không chạy lại CREATE TABLE trên bảng đã tồn tại:

```powershell
psql $env:TEST_MATCHING_DATABASE_URL -v ON_ERROR_STOP=1 -f internal/matching/postgres/migrations/001_init.sql
```

- Không thêm dependency vào go.mod/go.sum. Dùng database/sql và pgx đã có. Không hard-code DSN, không tự đọc `.env`. Timeout/pool do runtime/caller quản lý; đề xuất giữ deadline DB/service theo cấu hình integrator hiện có.

## Delta contract cần integrator công bố

| Endpoint/port/model | Hiện tại | Đích | Mapping/caller/test cần làm |
| --- | --- | --- | --- |
| POST `/internal/matching/requests` | Shared MatchingRequest chỉ rideId, mock 202 MATCHING | rideId + pickupLocation.lat/long bắt buộc; radiusMeters thiếu = 3000, cung cấp phải hữu hạn > 0 | Công bố OpenAPI/DTO/client và nguồn pickup thật ở caller trước khi nối Application.Request; không đổi Ride/fare bằng fixture |
| POST response | 202 `{rideId,status:MATCHING}` fixture | 202 `{rideId,status}` chỉ sau commit và đầy đủ acknowledgements, hoặc NO_DRIVERS/terminal | Test cả partial failure/retry, không trả 202 giả khi dependency lỗi |
| GET `/internal/matching/requests/{rideId}` | Luôn 200 MATCHING | 200 snapshot trạng thái job được cập nhật từ Ride; không dispatch | Map Application.Get, test không có job và Ride lỗi |
| Status | Chỉ MATCHING | MATCHING / NO_DRIVERS / MATCHED / CANCELLED | Đồng bộ enum OpenAPI/models/client/test; không coi MATCHED là Matching gán driver |
| Authentication/error envelope | Internal token, camelCase, X-Request-ID | Giữ nguyên tại boundary | Integrator giữ middleware và envelope hiện tại |

| Domain error (`errors.Is`) | HTTP/code đề xuất |
| --- | --- |
| `ErrValidation` hoặc lỗi decode input | 400 VALIDATION_ERROR |
| `ErrMatchingNotFound` | 404 MATCHING_NOT_FOUND — mới |
| `ErrRideNotFound` | 404 RIDE_NOT_FOUND |
| `ErrRideNotMatchable` | 409 RIDE_NOT_MATCHABLE — mới |
| `ErrRequestConflict` | 409 MATCHING_REQUEST_CONFLICT — mới |
| `ErrUnavailable` (kể cả DB, remote, ack sai, context) | 503 DEPENDENCY_UNAVAILABLE |

Không thay nullable driverId của contracts.Ride; Matching chỉ đọc snapshot, không có method gán driver hay ACCEPT. Integrator cập nhật shared error mapper/OpenAPI/catalog/caller/shared contract và smoke tests cùng nhau.

## Bằng chứng kiểm thử — kết quả hiện tại 2026-10-27

Các kết quả dưới đây đã chạy trong phiên kiểm tra trước ngay trong cuộc hội thoại này; lần rà soát yêu cầu chỉ đọc code, so sánh phạm vi/hash và cập nhật tài liệu, không chạy lại load suite.

Go 1.25.9, GCC 16.2.0, CGO_ENABLED=1 cho race. PATH/CC/TEMP/cache chỉ cấu hình trong phiên; không sửa cấu hình hệ thống. Dependency cần thiết đã có trong cache máy, chạy offline với GOPROXY=off; không sửa go.mod/go.sum. Tải trực tiếp vẫn lỗi DNS proxy.golang.org; cache mặc định không cho ghi lock.

| Kiểm tra | Kết quả thực tế | Giới hạn |
| --- | --- | --- |
| `go test -v ./internal/matching/... -count=1` | PASS | Unit dùng fake repository/remote ports |
| `go test -p=2 -race ./internal/matching/... -count=1 -timeout=60s` | PASS, exit 0 | Không thay thế PostgreSQL concurrency |
| `go vet ./...` | PASS, exit 0 | Toàn dự án |
| `go test -race ./... -timeout=120s` | PASS, exit 0 | GOFLAGS `-buildvcs=false -p=2`; không bật integration tag |
| `go test ./internal/contracttest -count=1` | PASS, exit 0 | Contract baseline giữ nguyên |
| `go build -buildvcs=false ./cmd/...` | PASS, exit 0 | Runtime Matching vẫn mock |
| `go test -p=2 -c -race -tags=integration -o .cache/matching-verify/matching-integration.test.exe ./internal/matching` | PASS, exit 0 | Đã biên dịch được với pgx thật |
| Chạy binary integration với TEST_MATCHING_DATABASE_URL chỉ DB test riêng | BLOCKED, exit 1 ở TestMain | Không kết nối được PostgreSQL; psql probe 127.0.0.1:55439 báo Permission denied / Windows 10013; Docker daemon chưa chạy |
| `make fmt-check vet test-race contract build` | Chưa chạy nguyên lệnh: thiếu Make | Các bước tương đương đã chạy trực tiếp, ghi kết quả từng bước ở trên/dưới |
| `python scripts/check_format.py` | FAIL | File baseline ngoài ownership có CRLF; không format protected files |
| `gofmt -l` trên 6 file Go mới | PASS ở lần kiểm tra trước | Code không thay đổi sau đó |
| `git diff --check` và so sánh protected paths với HEAD/BASE_SHA | PASS | Chỉ handoff tracked thay đổi; code mới Matching untracked; cgo-error.txt có trước và không thuộc delivery |

SQL-only checks từ lần triển khai trước trên PostgreSQL 18.6 single-user: migration, 15 trường hợp constraint sai, input/candidates ổn định, terminal không bị ghi đè, persistence sau mở lại backend đều PASS. Đây là kiểm tra SQL trực tiếp, không chứng minh Go adapter hoặc concurrency nhiều session đã PASS.

Unit test bao phủ validation/zero/biên/non-finite, dedup/thứ tự, không có ứng viên, retry/conflict, partial notification failure, lỗi DB/Ride/finder/notifier/ack, GET không dispatch, trạng thái terminal và context cancellation.

Integration test đã viết với PostgreSQL repository thật và fake remote ports: retry sau tạo lại application, hai application/pool dùng chung DB, input conflict, chỉ dispatch tập đã commit, terminal không bị request cũ ghi lùi, không giữ DB connection khi port ngoài chờ, timeout và constraints. **Chưa có bằng chứng PASS khi chạy các trường hợp này trên DB thật.**

## Đối chiếu yêu cầu — 2026-10-27

| Nhóm yêu cầu | Đối chiếu implementation/test | Kết luận |
| --- | --- | --- |
| Input pickup bắt buộc, radius chuẩn hóa | InputFields.Normalize/UnmarshalJSON phân biệt thiếu/null với số 0, mặc định radius 3000; Input.Validate kiểm tra hữu hạn/biên | Đáp ứng qua code và unit |
| Một job/tập ứng viên ổn định theo rideId | PK ride_id; CreateOrGet dùng transaction READ COMMITTED, INSERT ON CONFLICT rồi SELECT statement riêng; so sánh input | Có implementation và integration test; DB concurrency chưa xác minh |
| Context/kiểu thuần, phân lớp | Application/model không import Gin/SQL/Redis/broker; SQL ở postgres adapter; ba remote port chỉ đọc/tìm/yêu cầu notification | Đáp ứng |
| Chỉ tạo mới khi Ride REQUESTED | Request đọc Ride qua port trước finder/create; status terminal phản ánh snapshot Ride | Đáp ứng qua code và unit |
| Retry/conflict/lỗi một phần | Retry dùng danh sách cũ; NO_DRIVERS không tìm lại; lỗi notification/ack trả ErrUnavailable; tuple idempotent được giữ | Đáp ứng qua code và unit; Notification dedup là trách nhiệm port thật |
| Commit trước dispatch, không giữ transaction qua port ngoài | CreateOrGet chỉ trả sau Commit; transaction chỉ bao SQL, dispatch ở application | Có implementation/test; cần chạy PostgreSQL để chốt |
| Request cũ không ghi lùi terminal | AdvanceStatus chỉ cập nhật MATCHING/NO_DRIVERS; sau dispatch chỉ đọc lại job | Có implementation/test; race DB thật chưa xác minh |
| Bảo vệ baseline/ownership | Service/MockService/router/handler/DTO, cmd, Ride/Auth/platform/contracts/OpenAPI/dependencies/shared tests/work không có diff | Đáp ứng tại workspace hiện tại |
| Branch/base/handoff | Checkout riêng feat/matching-logic, BASE/HEAD 06142b4aa7b49adc2e0ec991a60bcfeaee142fcc; chưa commit; code/SQL khớp workspace review | Đã kiểm tra trực tiếp |
| Migration/transport | Chỉ schema matching, không FK/cross-service query/outbox/startup migration/HTTP-AMQP adapter/worker | Đáp ứng |

## Checklist trước review

- [x] Code mới chỉ trong Matching và handoff được giao; checkout riêng đã xác minh.
- [x] Mock/interface/router/handler/DTO/default runtime giữ nguyên.
- [x] Protected paths và dependency files không đổi; không ACCEPT/assignment/ghi Ride.
- [x] SQL chỉ schema matching; không FK/query xuyên service, startup migration hoặc outbox.
- [x] Unit/race/vet/contract/build đã có kết quả PASS; constructor/ports/SQL/contract delta đã mô tả.
- [ ] PostgreSQL integration thật xác minh persistence/concurrency/terminal race đã PASS.
- [ ] Format-check toàn dự án PASS — CRLF baseline cần integrator xử lý.
- [x] Chưa commit/push/merge; không đưa cgo-error.txt/cache/log/binary vào delivery.

## Dành cho integrator

Chưa accepted/integrated. Không có endpoint thật hoặc transport mới được bật.

Còn cần chạy PostgreSQL Matching integration với database riêng matching_test ở môi trường cho phép kết nối; xử lý format baseline thuộc ownership integrator. Sau khi review application/repository: đưa SQL vào migration có version/checksum, công bố contract pickup/status/error đích, cập nhật caller/client/handler và nguồn pickup thật, nối ports thật rồi bật runtime kèm regression/fault/smoke/benchmark theo integration.md. Không inject fake/stub làm fallback production.

Lệnh integration còn cần chạy:

```powershell
$env:TEST_MATCHING_DATABASE_URL = '<DSN database test riêng matching_test>'
go test -race -tags=integration ./internal/matching/... -count=1
```
