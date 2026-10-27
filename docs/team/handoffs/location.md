# Bàn giao Location — Người 1

## Trạng thái và Git

- Service / người phụ trách: Location / Person 1.
- Trạng thái: **READY_FOR_REVIEW** — application/repository, migration đề xuất và kiểm thử hoàn tất; chưa tích hợp runtime.
- BASE_SHA: `06142b4aa7b49adc2e0ec991a60bcfeaee142fcc` (`docs: assign service development and protect race baseline`), commit chung chứa instruction mới hơn baseline race.
- HEAD_SHA: working tree chưa commit, HEAD vẫn là BASE_SHA; chưa có commit bàn giao.
- Branch: `feat/location-logic`, tạo từ nhánh sạch `feature/khangpc`. Chưa push/PR/merge.
- File mới/sửa trong ownership:
  - `internal/location/application.go`
  - `internal/location/application_test.go`
  - `internal/location/postgres/repository.go`
  - `internal/location/postgres/repository_integration_test.go`
  - `internal/location/postgres/migrations/001_init.sql`
  - `docs/team/handoffs/location.md`

## Đã triển khai và chưa triển khai

- Application validate ID không trắng, tọa độ hữu hạn đúng biên, radius hữu hạn >0; tọa độ 0 và availability false hợp lệ.
- Upsert theo nhóm cột nguyên tử trong transaction tường minh: tọa độ không ghi availability, availability không ghi tọa độ. Driver mới bằng tọa độ mặc định unavailable; tạo bằng availability có cả hai tọa độ NULL. Giữ cả hai cập nhật khi chạy đồng thời trên row mới/cũ; chỉ trả thành công sau Commit.
- Nearby chỉ lấy available có tọa độ; Haversine R=6,371,000 m, clamp [0,1]; lọc inclusive và sort bằng khoảng cách thực rồi driver ID theo thứ tự chuỗi Go; chỉ làm tròn khi tạo output. Không kết quả trả `drivers:[]`.
- Service/MockService, `service.go`, router/handler/DTO và `cmd/location` còn nguyên. Application đáp ứng Service cũ nhưng **runtime vẫn mock**.
- Chưa có TTL, paging, PostGIS, cache, geocoding, lịch sử vị trí, active-ride filter hoặc remote call. Vị trí mới nhất theo thứ tự PostgreSQL thực hiện update; không timestamp thiết bị/loại bản tin đến muộn. Nearby quét danh sách available có tọa độ rồi tính trong application.

## Cách người tích hợp khởi tạo

Constructor và port trong `internal/location/application.go`:

```go
func NewApplication(repository Repository) *Application

type Repository interface {
    UpsertLocation(context.Context, string, contracts.Coordinates) error
    UpsertAvailability(context.Context, string, bool) error
    ListAvailable(context.Context) ([]DriverState, error)
}
```

`DriverState`: `DriverID string`, `Location *contracts.Coordinates` (nil khi chưa có vị trí), `Available bool`, `UpdatedAt time.Time`. Repository phải cập nhật theo cột nguyên tử, không read-modify-write cả row. Adapter chỉ liệt kê available có đủ tọa độ.

Adapter trong `internal/location/postgres/repository.go`: `func NewRepository(db *sql.DB) *Repository`. Ví dụ tại runtime, sau khi SQL đã được review/áp dụng:

```go
// import location "int3105-ride-share/internal/location"
// import locationpostgres "int3105-ride-share/internal/location/postgres"
// db: *sql.DB do runtime mở bằng pgx và quản lý lifecycle/pool.
repo := locationpostgres.NewRepository(db)
application := location.NewApplication(repo)
// Chỉ nối RegisterRoutes sau khi sửa error mapper/contract.
_ = application
```

Application nhận driver ID đã xác thực từ handler, không đọc token/header. Constructor không I/O/migrate. Context/timeout được truyền đến SQL; runtime chịu trách nhiệm đặt deadline.

Lỗi: `errors.Is(err, location.ErrValidation)` và `errors.Is(err, location.ErrDependencyUnavailable)`. Adapter trả `*location.DependencyError` có thông điệp cố định an toàn, không chứa SQL/DSN; `Unwrap` giữ nguyên nhân để `errors.Is/As` nhận context canceled/deadline. Lỗi ngoài hai nhóm này cần map INTERNAL_ERROR. Write lỗi không trả response thành công.

Transaction rollback nếu câu SQL lỗi/timeout trước Commit, gồm trường hợp đang chờ row lock. Mất kết nối đúng lúc Commit vẫn có thể có kết quả chưa xác định; lỗi dependency không phải bằng chứng đã rollback. Không tự retry kín trong repository; giữ semantics vị trí mới nhất theo thứ tự DB.

## Database và cấu hình

- SQL đề xuất: `internal/location/postgres/migrations/001_init.sql`, transaction chỉ tạo schema `location` và bảng `drivers`: driver_id text PK không trắng; lat/long double precision nullable cùng nhau, range check loại cả NaN/Inf; available NOT NULL DEFAULT false; updated_at timestamptz NOT NULL. Không index phụ hoặc FK.
- Dev áp dụng SQL rõ ràng vào DB test riêng; integration setup embed và thực thi chính file SQL. Constructor/replica không migrate. `IF NOT EXISTS` không xác minh shape bảng đã có; integrator phải review/version/checksum migration trước khi bật runtime.
- DSN env: `TEST_LOCATION_DATABASE_URL`; thiếu biến khi chọn tag integration là fail. Setup chỉ nhận database tên kết thúc `_test` (khuyến nghị `location_test`) trước khi migrate. Test tạo prefix ID ngẫu nhiên, cleanup từng ID sở hữu; không truncate/drop hoặc xóa dữ liệu khác. Constraint tests rollback kể cả nếu constraint thiếu.
- Lệnh kiểm tra: `go test -race -tags=integration ./internal/location/... -count=1`, với biến DSN trỏ DB test riêng.
- Không thêm dependency hoặc sửa config. Runtime cần inject DB/pool và deadline, không hard-code DSN hoặc RAM fallback.
- Không query/FK xuyên schema service, không truy cập Ride/Redis, không ảnh hưởng ACCEPT, không outbox/startup migration.

## Delta contract cần máy tích hợp công bố

| Endpoint/port/model | Hiện tại | Đích | Mapping/caller/test cần làm |
| --- | --- | --- | --- |
| POST `/drivers/location` | Echo | Cùng shape/status 200, sau khi lưu | Giữ JWT DRIVER/sub, required lat/long pointer tại boundary; 0 hợp lệ, thiếu/null không đổi thành 0. |
| PATCH `/drivers/availability` | Echo | Cùng shape/status 200, lưu độc lập | Required available pointer; false hợp lệ; có thể gọi trước vị trí. Retry cùng giá trị thành công, updated_at cập nhật. |
| GET `/internal/drivers/nearby` | Fixture một driver | Dữ liệu động hoặc `drivers:[]`, cùng shape/status 200 | Giữ internal token; lat/long bắt buộc. Default radius 3000 ở boundary khi thiếu; radius cung cấp phải hữu hạn >0, application không default 0. Cập nhật caller/shared test có chủ ý, không giữ expected fixture cố định. |
| Lỗi ba endpoint | Handler luôn map 500 | 400 VALIDATION_ERROR, 503 DEPENDENCY_UNAVAILABLE, 500 INTERNAL_ERROR | Dùng errors.Is hai sentinel; envelope hiện tại, thông điệp an toàn và X-Request-ID. Công bố 503 OpenAPI và cập nhật tests; 401/403 vẫn do middleware. |
| Readiness khi bật DB | Stateless | Integrator quyết định kiểm tra PostgreSQL | Công bố/kiểm thử cùng runtime DB thật; chưa đổi readiness ở delivery này. |

Không đổi shape/nullable field HTTP hoặc thêm enum; nullable tọa độ chỉ ở state nội bộ. Mapping vẫn phải giữ missing/null so với zero/false và loại NaN/Inf. Chưa sửa shared OpenAPI/contracts/client/error mapper/fixture.

## Bằng chứng kiểm thử

| Lệnh/kịch bản | Đã chạy? | Kết quả thực tế | Môi trường/giới hạn |
| --- | --- | --- | --- |
| `go test -race ./internal/location/...` | Có | PASS trên Linux | Go 1.25.9 linux/amd64 trong Docker; unit thường trên Windows cũng PASS |
| `go test -race -tags=integration ./internal/location/... -count=1 -v` | Có | PASS, đủ 5 nhóm PostgreSQL: persistence/independent updates, availability-first/recreation, concurrency, constraints, DB/context errors | PostgreSQL 16.10-alpine đúng digest Compose, DB location_test riêng trong container tạm, explicit DSN, không RAM |
| Integration không có TEST_LOCATION_DATABASE_URL | Có | Fail đúng yêu cầu với thông báo bắt buộc DSN, không skip | Kiểm tra âm sau khi suite thật pass; shell xác minh exit khác 0 |
| `make fmt-check vet test-race contract build` | Có | PASS tất cả target, exit 0 | Go 1.25.9 linux/amd64, snapshot LF; xem lưu ý CRLF phía dưới |
| `git diff --check`, diff --no-index --check file mới và ownership | Có | PASS | Đã tính cả 5 file untracked; protected paths, Service/MockService/handler/router/DTO không có diff |

Môi trường Windows ban đầu không có Go/Make trong PATH và Docker chưa chạy; dùng Go 1.25.9 portable trong cache và Docker để xác minh. Race binary Windows với GCC 8.1 lỗi `0xc0000139` trước khi chạy test; race trên Linux đã chạy thật và pass.

`make fmt-check` trực tiếp trên mount Windows fail vì core.autocrlf=true: các file baseline là i/lf nhưng w/crlf (đã xác minh bằng git ls-files --eol và gofmt -d). Không format file ngoài ownership. Bộ make được chạy lại trên snapshot tạm trong container: git archive HEAD (baseline LF), chép đúng 5 file Location mới rồi chạy nguyên các target. Không sửa formatter, expected, shared tests hoặc baseline để bỏ qua lỗi.

Sau kiểm thử, PostgreSQL trả count(*)=0 cho location.drivers; container/DB test tạm đã dừng và tự xóa. Không dùng hoặc xóa database/volume Ride của dự án.

Lần integration đầu phát hiện autocommit có thể hoàn tất câu ghi chờ row lock sau khi client timeout. Đã sửa adapter dùng transaction tường minh, giữ nguyên assertion; integration chạy lại pass. Trường hợp mất kết nối trong Commit vẫn có giới hạn kết quả chưa xác định như đã nêu trên.

Test đã viết: defaults/latest/independent upserts, availability trước vị trí, nhiều pool và tạo lại application, concurrent insert/update, SQL constraints, closed DB và row-lock timeout/context cancel; validation/mapping missing/null/zero/false, Haversine/đối cực/kinh tuyến 180, raw-distance boundary/sort/tie-break, JSON mảng rỗng. Fake chỉ trong unit test; không dùng RAM khi chọn integration.

## Checklist trước khi gửi review

- [x] Chỉ Location và handoff được giao.
- [x] Mock/interface/router/handler/DTO/default entrypoint giữ nguyên.
- [x] Không sửa Ride/Auth/platform/contracts/clients/shared tests/root migration/config/dependency/benchmark.
- [x] SQL chỉ schema location; không FK/query xuyên service/startup migration/outbox.
- [x] PostgreSQL thật đã xác minh persistence/concurrency; giới hạn ghi ở bảng test.
- [x] Constructor/SQL/contract delta/lệnh test đủ để integrator tiếp nhận.
- [x] Không commit .env/cache/binary/log/JWT, không tự commit/push/merge.

## Dành cho người tích hợp

- Commit review/merge: chưa có; thứ tự tích hợp Location → Notification → Matching.
- Contract/migration/wiring áp dụng: chưa; runtime vẫn mock.
- Regression/race/fault/smoke/benchmark hệ thống: thuộc integrator trước final integration; chưa tuyên bố có kết quả trong delivery này.
- Mốc: chờ review application/repository, chưa endpoint real integrated hoặc transport integrated.
- Còn lại: review/version migration, công bố API/catalog/tests động và error mapping, cấu hình DB/deadline/readiness, chuyển runtime có chủ ý rồi chạy đầy đủ checklist integration. Không thêm remote work vào ACCEPT.
