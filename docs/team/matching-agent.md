# Prompt cho agent Người 2 — Matching

Copy nguyên khối dưới vào agent của người phụ trách Matching. Có thể bắt đầu ngay bằng fake remote ports; không cần đợi Location/Notification có endpoint thật.

```text
Bạn phụ trách Người 2 / Matching của repo int3105-ride-share.

Đọc theo thứ tự:
1. AGENTS.md và internal/matching/AGENTS.md.
2. docs/team/README.md.
3. docs/team/matching-requirements.md.
4. docs/team/location-requirements.md và docs/team/notification-requirements.md
   để hiểu port đầu ra/đặc tính idempotency; không triển khai hai service này.
5. docs/development.md, docs/api-catalog.md, docs/openapi/matching.yaml.
6. docs/team/handoff-template.md và docs/team/handoffs/matching.md.

Triển khai application/repository thật theo matching-requirements.md:
- Input nghiệp vụ có rideId, pickupLocation và radius đã chuẩn hóa.
- Schema matching lưu một job/input/tập ứng viên ổn định theo rideId.
- Các port đọc Ride, tìm driver và yêu cầu Notification nhận context/kiểu thuần.
- Chỉ job mới trên Ride REQUESTED; trạng thái MATCHING/NO_DRIVERS/MATCHED/CANCELLED.
- Retry không đổi tập ứng viên; input khác conflict; partial notification failure
  trả unavailable và retry qua các yêu cầu idempotent.
- Không giữ transaction DB khi gọi port ngoài; không dispatch tập chưa commit;
  request cũ không được ghi lùi terminal status.

Matching chỉ đề xuất tài xế và lưu yêu cầu thông báo. KHÔNG gán tài xế thắng,
KHÔNG gọi ACCEPT, KHÔNG ghi DB Ride và KHÔNG sửa thuật toán race condition.
Ride chưa có pickup: không giả tọa độ, không lấy từ fare_123, không sửa Ride.
Định nghĩa model/port application nội bộ mới khi shared MatchingRequest chưa có
pickup; người tích hợp chịu trách nhiệm công bố contract và map handler/client sau.

Chỉ được sửa/thêm file trong internal/matching/ và điền
docs/team/handoffs/matching.md. Không sửa AGENTS.md/instruction/requirements.
Giữ interface/MockService/router/handler/DTO đang chạy tương thích baseline;
thêm implementation mới để test trực tiếp. Chưa tự đổi cmd/matching hoặc default
runtime. Fake remote ports chỉ dùng cho test, không làm fallback production.

Không sửa cmd/, service khác, platform/shared contracts/clients, shared tests,
root migrations/, docs/openapi/, fixtures/, Compose/Docker, go.mod/go.sum,
Makefile, CI, scripts/notebooks/benchmark hoặc work/. Không viết HTTP/RabbitMQ
adapter liên service hoặc worker; máy hiện tại làm transport và wiring cuối.
Không tự tạo thêm agent.

Baseline race được bảo vệ: 8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9.
Lấy commit tài liệu chung được người tích hợp thông báo, ghi BASE_SHA thực tế,
làm trên branch feat/matching-logic/worktree riêng. Không reset/clean thay đổi
của người khác hoặc push main.

Đặt SQL tại internal/matching/postgres/migrations/001_init.sql, chỉ schema matching;
test DB riêng matching_test. Không FK/query trực tiếp bảng service khác, không
auto-migrate startup, không sửa root migration runner hoặc thêm outbox.

Kiểm tra:
  go test -race ./internal/matching/...
  TEST_MATCHING_DATABASE_URL=<DSN database test riêng> go test -race -tags=integration ./internal/matching/... -count=1
  make fmt-check vet test-race contract build
  git diff --check
Format chỉ Go files thuộc phạm vi mình. Test phải dùng PostgreSQL thật cho
persistence/concurrency và fake port có thể kiểm soát lỗi cho dependency ngoài.

Điền handoff: constructor/port/model đủ để người tích hợp khởi tạo, SQL,
input pickup bắt buộc ở runtime đích, status/error mới, kịch bản retry/lỗi một phần,
base/head SHA, test thực sự đã chạy và giới hạn. Không sửa OpenAPI chung hoặc
test fixture cũ để tránh công bố thay đổi API ngoài kế hoạch.

Kết thúc với báo cáo công việc đã hoàn thành và phần cần tích hợp; không tuyên bố
HTTP endpoint thật đã bật. Chỉ commit/push branch service khi chủ task đã yêu cầu
trong phiên, không tự merge. Thiếu shared dependency thì đề xuất trong handoff
và tiếp tục phần độc lập, không tự sửa vùng bảo vệ.
```

Nguồn yêu cầu: [Matching](matching-requirements.md). Giới hạn chung: [phân công](README.md). Trình tự ghép hệ thống: [integration](integration.md).
