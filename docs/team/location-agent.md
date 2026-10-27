# Prompt cho agent Người 1 — Location

Copy nguyên khối dưới vào agent trên máy/branch của người phụ trách Location. Prompt này giao mốc application/repository, không giao việc tích hợp hệ thống.

```text
Bạn phụ trách Người 1 / Location của repo int3105-ride-share.

Đọc theo thứ tự:
1. AGENTS.md và internal/location/AGENTS.md.
2. docs/team/README.md.
3. docs/team/location-requirements.md.
4. docs/development.md, docs/api-catalog.md và docs/openapi/location.yaml.
5. docs/team/handoff-template.md và docs/team/handoffs/location.md.

Thực hiện đầy đủ yêu cầu Location thật trong docs/team/location-requirements.md:
- Thêm application nhận repository interface, PostgreSQL adapter schema location,
  SQL đề xuất và unit/integration tests.
- Lưu vị trí mới nhất, availability độc lập; nearby lọc/tính Haversine/sort thật.
- Không có TTL, PostGIS, Redis cache hoặc side effect sang service khác.

Chỉ được sửa/thêm file trong internal/location/ và điền
docs/team/handoffs/location.md. Không sửa AGENTS.md/instruction/requirements.
Giữ nguyên hành vi router, handler, DTO, Service/MockService hiện tại để toàn bộ
fixture/contract test baseline vẫn chạy; thêm implementation/model/port ở file mới.
Không phá interface cũ, không sửa shared contracts/client để làm code mình compile.
Chưa tự đổi cmd/location hoặc bật default implementation thật.

Không sửa cmd/, internal/ride/, internal/auth/, internal/platform/,
internal/contracts/, internal/clients/, internal/integration/, internal/contracttest/,
root migrations/, docs/openapi/, fixtures/, Compose/Docker, go.mod/go.sum,
Makefile, CI, scripts, notebooks hoặc benchmark. Không đụng work/ và thay đổi
của người khác. Không tự tạo thêm agent.

Baseline race cần bảo vệ: 8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9.
Làm từ commit tài liệu chung được người tích hợp thông báo (có thể mới hơn baseline),
ghi lại BASE_SHA thực tế. Dùng branch feat/location-logic hoặc worktree riêng;
nếu đang có thay đổi của người khác thì giữ nguyên, không reset/clean/force checkout.

SQL đề xuất nằm trong internal/location/postgres/migrations/001_init.sql,
chỉ tạo/cập nhật schema location. Dùng database test riêng location_test;
không đọc/ghi ride.rides, không tự migrate trong startup, không sửa bootstrap Ride.
Không được dùng RAM thay cho PostgreSQL khi chạy integration test.

Kiểm tra:
  go test -race ./internal/location/...
  TEST_LOCATION_DATABASE_URL=<DSN database test riêng> go test -race -tags=integration ./internal/location/... -count=1
  make fmt-check vet test-race contract build
  git diff --check
Format chỉ những file Go thuộc phạm vi mình, không chạy formatter sửa toàn repo.
Nếu thiếu dependency/mapping/contract chung, ghi đề xuất chính xác vào handoff;
tiếp tục phần application/repository độc lập, không tự nới quyền sửa shared files.

Hoàn tất docs/team/handoffs/location.md với base/head SHA, danh sách file,
constructor/model/port, SQL và cách áp dụng, mapping HTTP đích, test đã chạy,
kết quả và giới hạn. Phân biệt test chưa chạy với test pass. Không tuyên bố endpoint
thật đã được bật vì entrypoint vẫn mock. Không đổi fixture để làm test qua.

Cuối cùng báo cáo ngắn kết quả và việc cần người tích hợp làm. Chỉ commit/push
branch service khi chủ task đã yêu cầu trong phiên; không push main hoặc tự merge.
```

Nguồn yêu cầu: [Location](location-requirements.md). Giới hạn chung: [phân công](README.md). Phần máy tích hợp làm sau: [integration](integration.md).
