# Prompt cho agent Người 3 — Notification

Copy nguyên khối dưới vào agent của người phụ trách Notification. Mốc này là lưu yêu cầu thông báo thật, chưa gửi push qua nhà cung cấp.

```text
Bạn phụ trách Người 3 / Notification của repo int3105-ride-share.

Đọc theo thứ tự:
1. AGENTS.md và internal/notification/AGENTS.md.
2. docs/team/README.md.
3. docs/team/notification-requirements.md.
4. docs/development.md, docs/api-catalog.md, docs/openapi/notification.yaml,
   docs/events.md (topology hiện tại chưa có publisher/consumer).
5. docs/team/handoff-template.md và docs/team/handoffs/notification.md.

Thêm application/repository PostgreSQL thật theo notification-requirements.md:
- Lưu yêu cầu RIDE_REQUEST, ID thật, trạng thái QUEUED sau commit.
- Unique theo (rideId,driverId,type), retry/concurrent request trả cùng ID.
- Có nghiệp vụ đọc notification theo ID; đề xuất mapping GET internal trong handoff.
- Giữ dữ liệu sau khi tạo lại application/repository, không fake success khi DB lỗi.

QUEUED chỉ có nghĩa đã lưu DB. Không thêm provider, delivery worker, push,
email/SMS/websocket, RabbitMQ adapter, HTTP call hoặc trạng thái SENT/DELIVERED.
Không xác minh ID bằng cách query DB Ride/Auth hoặc gọi ACCEPT.

Chỉ sửa/thêm file trong internal/notification/ và điền
docs/team/handoffs/notification.md. Không sửa AGENTS.md/instruction/requirements.
Giữ nguyên MockService/router/handler/DTO/interface đang chạy, gồm fixture
notification_123. Thêm implementation/model/port mới để test trực tiếp.
Người tích hợp sẽ công bố GET/mã lỗi mới, map handler và đổi cmd/notification sau.

Không sửa cmd/, service khác, internal/platform/contracts/clients,
internal/integration/contracttest, root migrations/, docs/openapi/, fixtures/,
Compose/Docker, go.mod/go.sum, Makefile, CI, scripts, notebooks, benchmark, work/.
Không tự tạo thêm agent. Không đổi shared fixture hoặc expected để làm test qua.

Baseline race: 8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9.
Lấy commit tài liệu chung được người tích hợp thông báo, ghi BASE_SHA thực tế,
làm trên feat/notification-logic/worktree riêng. Giữ nguyên thay đổi của người khác;
không reset/clean/force checkout hoặc push main.

SQL: internal/notification/postgres/migrations/001_init.sql, chỉ schema notification.
Test bằng DB riêng notification_test; không FK sang service khác, không auto-migrate
startup và không sửa root migration runner. Unique constraint phải là cơ chế chống
trùng cuối cùng, không chỉ map/mutex trong một process hoặc SELECT rồi INSERT.

Kiểm tra:
  go test -race ./internal/notification/...
  TEST_NOTIFICATION_DATABASE_URL=<DSN database test riêng> go test -race -tags=integration ./internal/notification/... -count=1
  make fmt-check vet test-race contract build
  git diff --check
Format chỉ Go files thuộc phạm vi mình. Integration dùng PostgreSQL thật, hai
application cùng DB và request đồng thời; thiếu DSN khi chọn tag phải báo lỗi.

Điền handoff với constructor/repository/model, SQL, GET đích và
NOTIFICATION_NOT_FOUND cần công bố, cách chạy test, base/head SHA, các test đã
thực sự chạy và giới hạn. Không gọi mốc này là delivery/push hoàn chỉnh hoặc
endpoint thật đã bật. Shared contract/dependency cần đổi thì ghi đề xuất để máy
tích hợp xử lý; không tự sửa ngoài phạm vi.

Kết thúc bằng báo cáo ngắn. Chỉ commit/push branch service khi chủ task đã yêu cầu
trong phiên; không tự merge hoặc push main.
```

Nguồn yêu cầu: [Notification](notification-requirements.md). Giới hạn chung: [phân công](README.md). Công việc sau bàn giao: [integration](integration.md).
