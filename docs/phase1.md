# Đối chiếu yêu cầu Pha 1

Nguồn đề bài: `work/de_bai.md` trong workspace của nhóm.

| Yêu cầu | Implementation / bằng chứng |
| --- | --- |
| REST JSON, POST/GET/DELETE | Ride tạo, đọc, hủy mềm; smoke có xác thực |
| OpenAPI | 5 spec trong docs/openapi; validate schema và response thật trong contract tests |
| API → nghiệp vụ → data access | Handler/service/repository port và PostgreSQL data mapper; dependency nối tại runtime |
| Nghiệp vụ không import web/DB library | service.go dùng context, contracts và port; adapter có import infrastructure |
| Hỗ trợ login | bcrypt credential fixture, JWT ký HS256, claims và thời hạn được kiểm tra |
| Có GET và POST cần xác thực | GET /rides/{id}, POST /rides; middleware dùng chung |
| Docker | 5 service, migration riêng, PostgreSQL/Redis/RabbitMQ; profile infra/full |
| README kiến trúc/đặc tả | README, catalog, development, event/benchmark docs |
| GitHub công khai, commit cẩn thận | Repo đã cấu hình origin; public visibility và push/commit do nhóm quản lý, chưa thay đổi từ phiên triển khai này |
| Kiểm thử tải Kaggle CPU | Đã có notebook/bootstrap/load generator; còn cần Save & Run All trên tài khoản Kaggle |

Auth register có chủ ý mock, không lưu tài khoản mới. Login fixture là cơ chế chạy thật cho Pha 1; không mô tả hệ thống này là quản lý tài khoản production. Location/Matching/Notification có application/repository thật đã kiểm thử nhưng HTTP runtime vẫn mock; RabbitMQ chỉ có topology. Luồng nhận cuốc không phát event và không tự đồng bộ Matching.

## Tiền đề Pha 2

Baseline có hai phần tách biệt: Reduce contention bằng Redis và Data transaction bằng PostgreSQL. Cả `sql` và `redis_sql` đúng dữ liệu; benchmark so sánh trên cùng phần cứng và workload. Chỉ bảo vệ một cuốc/một tài xế thắng, chưa có giới hạn một tài xế/một cuốc hoặc tối ưu matching.

## Kiểm chứng local

Đã chạy các kiểm tra sau trên workspace triển khai:

- `make fmt-check vet test-race contract build`.
- `python3 scripts/native_check.py`: native PostgreSQL 16.14 / Redis 8.6.3, race/fault tests và smoke đủ 5 service.
- Compose full stack với image/digest đã khóa: healthy, smoke thành công và xác minh 3 binding RabbitMQ.
- `make integration` với PostgreSQL 16.10 / Redis 7.4.2 trong Compose: hai replica, cạnh tranh, TTL, kill process giữ khóa, degraded readiness và lỗi DB.
- Bootstrap source có checksum đã build thành công trên Linux amd64 chạy root; native integration/race/fault và HTTP smoke qua với Go 1.25.9, PostgreSQL 16.10, Redis 7.4.2, PostgreSQL chạy dưới nobody.
- Benchmark local hai mode, concurrency 2/10/50/100, 10 nhóm đo, 2 warm-up, 2 lần lặp: không có lỗi dữ liệu/hệ thống, đã xuất và kiểm tra biểu đồ PNG/SVG.

Kết quả local dùng để kiểm chứng implementation và công cụ, không phải số liệu Kaggle hoặc kết luận hiệu năng chính thức. CPU, phiên bản/cấu hình, dữ liệu thô và binary hash nằm trong artifacts của từng run. Chưa có Save & Run All trên tài khoản Kaggle.
