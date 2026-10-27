# Phân công 3 service và máy tích hợp

Đây là kế hoạch giao việc phát triển tiếp, không phải mô tả tính năng đã chạy. Baseline được bảo vệ là commit `8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9` (`feat: implement race-safe ride acceptance baseline`). Hiện Location/Matching/Notification vẫn mock, Ride chạy PostgreSQL/Redis thật và Auth dùng fixture/JWT.

## Giao instruction nào cho ai

| Người | Service / branch đề xuất | Yêu cầu | Prompt cho agent |
| --- | --- | --- | --- |
| Người 1 | Location / `feat/location-logic` | [Location](location-requirements.md) | [Copy prompt Location](location-agent.md) |
| Người 2 | Matching / `feat/matching-logic` | [Matching](matching-requirements.md) | [Copy prompt Matching](matching-agent.md) |
| Người 3 | Notification / `feat/notification-logic` | [Notification](notification-requirements.md) | [Copy prompt Notification](notification-agent.md) |
| Máy hiện tại | Người tích hợp | Ride/race, Auth, nền tảng, contract chung, môi trường, benchmark/Kaggle và tích hợp cuối | [Quy trình tích hợp](integration.md) |

Mỗi người clone repo trên máy mình hoặc dùng worktree/branch riêng; không làm đồng thời trong một working directory. Đọc tài liệu yêu cầu rồi copy nguyên khối prompt tương ứng vào agent. Không tự tạo thêm agent để chia việc. Tên Người 1–3 có thể được thay bằng tên thành viên khi giao, không làm thay đổi ownership.

## Phạm vi được sửa

| Vai trò | Được triển khai | Chỉ người tích hợp sửa |
| --- | --- | --- |
| Người 1 | Implementation thật, domain/model/port, PostgreSQL adapter, SQL và test trong `internal/location/`; [handoff Location](handoffs/location.md) | Các file ngoài phạm vi, wiring `cmd/location`, OpenAPI đang công bố và contract dùng chung |
| Người 2 | Implementation thật, domain/model/port, PostgreSQL adapter, SQL và test trong `internal/matching/`; [handoff Matching](handoffs/matching.md) | Các file ngoài phạm vi, wiring `cmd/matching`, HTTP/RabbitMQ adapter liên service và contract dùng chung |
| Người 3 | Implementation thật, domain/model/port, PostgreSQL adapter, SQL và test trong `internal/notification/`; [handoff Notification](handoffs/notification.md) | Các file ngoài phạm vi, wiring `cmd/notification`, transport/delivery và contract dùng chung |
| Người tích hợp | Toàn bộ phần đã có, review và tích hợp các service mới | Có trách nhiệm giữ các bất biến và kết quả kiểm thử baseline |

Trong service của mình, **giữ nguyên hành vi `MockService`, router, handler và DTO hiện tại trong đợt bàn giao độc lập**. Thêm implementation thật ở các file mới, test gọi trực tiếp implementation đó. Có thể thêm interface/model nội bộ; không phá chữ ký interface cũ mà test/client hiện tại đang dùng. Không sửa `AGENTS.md` hoặc tài liệu yêu cầu để tự nới phạm vi. Ghi các thay đổi HTTP cần thiết vào file handoff; người tích hợp sẽ thực hiện sau khi công bố contract đích.

Ví dụ: Matching có thể dùng input nghiệp vụ riêng chứa pickup thay vì sửa `contracts.MatchingRequest` đang chỉ có rideId. Đây là application implementation để bàn giao, chưa phải endpoint thật đã được bật. Các SQL adapter được phép làm I/O PostgreSQL; các port tới service khác chỉ được fake trong test, chưa có HTTP/AMQP I/O liên service.

## Vùng bảo vệ

Ba người không chỉnh các phần sau trong PR service:

- `cmd/`, `internal/ride/`, `internal/auth/`, `internal/platform/`, `internal/contracts/`, `internal/clients/`.
- `migrations/`, `infra/`, `compose.yaml`, `Dockerfile`, `.env.example`, `.go-version`, `go.mod`, `go.sum`, `Makefile`, `.github/`.
- `internal/integration/`, `internal/contracttest/`, `scripts/`, `notebooks/`, `fixtures/` và `docs/openapi/`.
- Root/shared documentation, instruction và `work/`, ngoài file handoff đã được giao.

Đặc biệt không thay conditional UPDATE/transaction, Redis gate, TTL/release token, fallback SQL, auth/ownership, mã lỗi ACCEPT hoặc workload/metric benchmark; không thêm HTTP/RabbitMQ call vào đường ACCEPT. Không thêm ràng buộc một tài xế/một cuốc trong baseline này. Không giảm test hoặc đổi expected để hợp thức hóa sai lệch.

## Dữ liệu và môi trường

Mỗi service thật dùng **PostgreSQL schema riêng**: `location`, `matching`, `notification`. Không đọc/ghi `ride.rides`, không tạo foreign key xuyên service, không thêm CouchDB hoặc dùng Redis key của Ride. Mọi truy cập Ride đều đi qua port đọc snapshot.

SQL đề xuất đặt tại `internal/<service>/postgres/migrations/001_init.sql`, chỉ tác động schema của service. Dev áp dụng rõ ràng vào **database test riêng** để kiểm tra; không sửa bootstrap Ride, không tự migrate lúc application startup. Người tích hợp đưa SQL vào migration runner có version/checksum trước khi bật runtime thật. Không để runner tạo một bảng có shape chưa được quản lý bằng migration.

Go 1.25.9 và dependency hiện tại đã đủ cho nghiệp vụ, `database/sql`/pgx và HTTP scaffold. Nếu cần dependency mới, ghi tên/version/lý do trong handoff; không tự sửa go.mod/go.sum. Dùng repository/port injection để kiểm thử mà không cần chờ transport của người khác.

## Trình tự làm việc

1. Người tích hợp publish commit tài liệu này; mỗi người tạo branch từ commit chung đã thông báo, không chỉ checkout baseline cũ vì baseline cũ chưa chứa instruction mới.
2. Mỗi dev đọc requirement, triển khai application/repository thật và test riêng; mock/default entrypoint tiếp tục hoạt động như baseline.
3. Hoàn tất handoff, commit/push branch service hoặc mở PR, không push thẳng nhánh tích hợp/main. Không tự merge.
4. Người tích hợp review phạm vi và test, tích hợp lần lượt **Location → Notification → Matching**; công bố contract và sửa handler/wiring trên máy này.
5. Người tích hợp nối HTTP/RabbitMQ trong công việc riêng, chạy regression/race/benchmark trước khi chốt bản cuối. Không trộn việc nối RabbitMQ vào PR nghiệp vụ của ba người.

Location và Notification không phụ thuộc nhau để bắt đầu. Matching bắt đầu ngay bằng fake port và PostgreSQL riêng, không phải chờ hai service kia hoặc contract Go chung thay đổi. Fake port phục vụ test, không được dùng làm fallback im lặng trong runtime thật sau tích hợp.

## Hoàn tất bàn giao

Mỗi người điền file handoff của mình theo [mẫu](handoff-template.md), gồm base/head SHA, phạm vi file, SQL, interface/contract đích, test đã chạy, kết quả và phần còn cần tích hợp. Thay đổi ngoài ownership phải được tách khỏi PR, không giải quyết bằng cách sửa luôn service khác.

Chưa yêu cầu ba người hoàn tất endpoint runtime thật: mốc của họ là **application thật + repository thật + test + migration đề xuất + handoff**. Mốc **endpoint thật hoạt động cùng các service** thuộc người tích hợp. Xem [integration.md](integration.md) để biết tiêu chí đóng từng mốc.
