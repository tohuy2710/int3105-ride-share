# Người 3 — yêu cầu Notification thật

Trạng thái: **đặc tả đích**, chưa có delivery/push provider. Đọc [phân công](README.md), [prompt](notification-agent.md), [OpenAPI hiện tại](../openapi/notification.yaml) và [event contract hiện tại](../events.md).

## Mục tiêu

Thêm application và PostgreSQL repository trong `internal/notification/` để nhận, lưu và truy vấn yêu cầu thông báo. `QUEUED` có nghĩa yêu cầu đã lưu bền trong DB, không có nghĩa RabbitMQ đã nhận hoặc thiết bị đã nhận. Giữ MockService/HTTP/default entrypoint hiện tại; người tích hợp nối endpoint thật sau.

Không thêm provider, worker, retry delivery, websocket, email, SMS, RabbitMQ consumer hoặc HTTP call tới service khác. Không thêm trạng thái `SENT`/`DELIVERED` hoặc xác minh ride/driver bằng cách truy cập bảng service khác.

## Contract HTTP đích để bàn giao

| Endpoint | Input | Thành công | Quyền |
| --- | --- | --- | --- |
| POST `/internal/notifications/ride-requests` | `{rideId,driverId,type:"RIDE_REQUEST"}` | 202 `{notificationId,rideId,driverId,type:"RIDE_REQUEST",status:"QUEUED"}` | Internal token |
| GET `/internal/notifications/{notificationId}` — mới | path ID | 200 cùng snapshot 5 trường ở trên | Internal token |

Lần đầu sinh notification ID duy nhất, không rỗng; không còn dùng ID cố định `notification_123` trong implementation thật. Mock vẫn trả fixture cũ. Không thêm timestamp hoặc trường nội bộ vào output đích.

| Tình huống | Kết quả đích |
| --- | --- |
| Input/path ID rỗng, sai type | 400 `VALIDATION_ERROR` |
| GET ID chưa tồn tại | 404 `NOTIFICATION_NOT_FOUND` — code mới cần người tích hợp công bố/map |
| DB lỗi, timeout hoặc commit không xác định | 503 `DEPENDENCY_UNAVAILABLE`, không trả acknowledgement thành công giả |
| Internal token thiếu/sai | 401 `UNAUTHORIZED`, middleware chung |

Application trả lỗi nghiệp vụ không phụ thuộc HTTP; người tích hợp cập nhật error mapper/OpenAPI/client chung. Không tự sửa platform vì code mới chưa được map trong baseline.

## Idempotency và lưu dữ liệu

- Khóa nghiệp vụ là `(rideId,driverId,type)`. Cùng bộ ba luôn trả lại cùng notification ID/status, kể cả request đồng thời, sau restart hoặc client không nhận được response đầu tiên.
- Khác ride hoặc driver tạo yêu cầu khác. Hiện payload chỉ gồm bộ ba này nên không có payload bổ sung để overwrite/conflict.
- Dựa vào unique constraint PostgreSQL và upsert/read phù hợp; không chỉ kiểm tra bằng SELECT trước INSERT, không chỉ dùng mutex hoặc map trong process.
- Chỉ trả 202 sau khi transaction/statement ghi đã commit. Retry không đổi createdAt hoặc tạo thêm row. GET không gây side effect.
- Chỉ validate ID không rỗng và type. Không bắt buộc UUID, không tạo FK hoặc query sang Ride/Auth để xác minh tồn tại.

SQL đề xuất tại `internal/notification/postgres/migrations/001_init.sql`, schema `notification`, bảng `requests` tối thiểu:

| Cột | Yêu cầu |
| --- | --- |
| id | text PK, ID ứng dụng sinh |
| ride_id, driver_id | text NOT NULL, không rỗng |
| type | text NOT NULL, chỉ RIDE_REQUEST |
| status | text NOT NULL, chỉ QUEUED trong phạm vi này |
| created_at | timestamptz NOT NULL |

Thêm UNIQUE `(ride_id,driver_id,type)`; không tạo outbox, delivery log, bảng users hoặc bảng broker message. Repository nhận DB qua constructor; service nhận repository interface; migration chạy riêng, không ở startup.

## Test bắt buộc

- Enqueue tạo row và ID thật; GET trả đúng snapshot; type/ID sai và ID không tồn tại.
- Retry tuần tự giữ nguyên ID và chỉ một row; nhiều request đồng thời qua hai application/repository cùng DB vẫn chỉ một row/ID.
- Khác driver/ride tạo ID khác. Tạo lại application/repository vẫn đọc và deduplicate được dữ liệu.
- DB lỗi, context timeout và rollback không trả thành công giả; GET không đổi dữ liệu.
- Không có lời gọi external provider/HTTP/AMQP trong implementation; fixture mock và test baseline không đổi.
- PostgreSQL integration dùng build tag `integration`, DSN `TEST_NOTIFICATION_DATABASE_URL`; thiếu DSN khi chạy tag phải báo lỗi, không fallback RAM. Cleanup chỉ row của test.

## Chạy và bàn giao

Tạo database test riêng một lần; nếu đã tồn tại và thuộc dev thì dùng lại:

```bash
make infra-up
docker compose --profile infra exec -T postgres createdb -U ride notification_test
go test -race ./internal/notification/...
TEST_NOTIFICATION_DATABASE_URL='postgres://ride:ride_local@localhost:5432/notification_test?sslmode=disable' \
  go test -race -tags=integration ./internal/notification/... -count=1
make fmt-check vet test-race contract build
git diff --check
```

Integration setup áp dụng SQL service rõ ràng vào DB test, không sửa migration runner hiện tại. Go trực tiếp không tự load `.env`; thay DSN theo host port local. Điền [handoff Notification](handoffs/notification.md), nêu rõ endpoint GET và mã lỗi mới cần tích hợp. Chưa đổi router, default entrypoint, shared client hoặc OpenAPI đang công bố.
