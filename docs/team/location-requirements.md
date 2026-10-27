# Người 1 — yêu cầu Location thật

Trạng thái: **đặc tả đích để triển khai**, chưa được bật trong entrypoint. Đọc [phân công và phạm vi](README.md), [prompt](location-agent.md), [API hiện tại](../openapi/location.yaml) và [catalog](../api-catalog.md).

## Mục tiêu và đầu ra

Thêm application thật và PostgreSQL repository trong `internal/location/`. Lưu vị trí gần nhất và availability của từng tài xế; nearby trả dữ liệu đã lưu thay vì fixture. Giữ mock/interface HTTP đang chạy; người tích hợp chuyển handler sang implementation mới sau khi nhận bàn giao.

Nghiệp vụ không import Gin, database/sql, pgx, Redis hoặc RabbitMQ. Nhận driver ID đã được middleware xác thực qua tham số; không tự đọc token/header trong application. Không gọi Ride, Auth, Matching, Notification hoặc sửa availability theo trạng thái Ride.

## Hành vi HTTP đích

Đây là yêu cầu cho người tích hợp map vào API sau, không phải yêu cầu dev sửa router ngay.

| Endpoint | Input | Thành công | Quyền |
| --- | --- | --- | --- |
| POST `/drivers/location` | `{lat,long}` | 200 `{driverId:<JWT sub>,lat:<input>,long:<input>}` sau khi lưu | DRIVER |
| PATCH `/drivers/availability` | `{available}` | 200 `{driverId:<JWT sub>,available:<input>}` sau khi lưu | DRIVER |
| GET `/internal/drivers/nearby` | `lat`, `long`, `radiusMeters` | 200 `{drivers:[{driverId,location:{lat,long},distanceMeters}]}` | Internal token |

- Lat trong [-90,90], long trong [-180,180]; số 0 hợp lệ. Thiếu tọa độ không được hiểu thành 0.
- `available:false` hợp lệ, thiếu trường không hợp lệ. ID tài xế không rỗng.
- Nearby yêu cầu lat/long; thiếu radius mặc định 3000 m ở HTTP boundary; radius cung cấp phải hữu hạn và >0. Application nhận radius đã chuẩn hóa, không dùng 0 làm giá trị mặc định.
- Lỗi input: `400 VALIDATION_ERROR`; lỗi DB: `503 DEPENDENCY_UNAVAILABLE`; lỗi ngoài dự kiến: `500 INTERNAL_ERROR`. 401/403 do middleware chung xử lý. Lỗi dùng envelope hiện tại; không lộ SQL/DSN.

## Quy tắc lưu và truy vấn

1. Cập nhật tọa độ là upsert theo driver ID, chỉ đổi lat/long và updatedAt; không đổi availability đã có. Driver chưa có bản ghi mặc định `available=false`.
2. Cập nhật availability là upsert chỉ đổi availability và updatedAt. Cho phép thao tác này trước khi có tọa độ; khi đó cả lat và long là null.
3. Hai thao tác đồng thời trên cùng driver không được làm mất cập nhật của trường còn lại. Dùng cập nhật theo cột trong repository, không ghi đè toàn bộ row từ snapshot cũ.
4. Nearby chỉ lấy driver available và có đủ tọa độ. Driver chưa có tọa độ bị loại dù available=true.
5. Tính khoảng cách Haversine với bán kính Trái Đất 6,371,000 m; clamp hệ số trung gian về [0,1] để tránh lỗi số học ở điểm đối cực. Lọc bằng khoảng cách chưa làm tròn `distance <= radius`.
6. Sắp xếp tăng theo khoảng cách chưa làm tròn, rồi driver ID theo thứ tự chuỗi. `distanceMeters` là số nguyên làm tròn gần nhất khi trả response. Không có kết quả trả `drivers:[]`, không trả null.
7. Chưa có TTL, paging, PostGIS, geocoding, cập nhật theo lịch hoặc lọc theo active ride. Mỗi driver chỉ giữ vị trí mới nhất theo thứ tự DB thực hiện update; chưa hỗ trợ timestamp thiết bị hoặc loại bản tin đến muộn.

## Dữ liệu tối thiểu

SQL đề xuất: `internal/location/postgres/migrations/001_init.sql`. Schema `location`, bảng `drivers`:

| Cột | Yêu cầu |
| --- | --- |
| driver_id | text PK, không rỗng |
| lat, long | double precision nullable; null cùng nhau; giới hạn tọa độ khi có giá trị |
| available | boolean NOT NULL DEFAULT false |
| updated_at | timestamptz NOT NULL |

Không có FK sang tài khoản/Ride, bảng lịch sử vị trí hoặc Redis cache. Chỉ thêm index nếu truy vấn thực tế cần, không xây hệ thống GIS cho bài tối thiểu. Repository nhận DB qua constructor; application nhận repository interface. Không hard-code DSN và không tự migrate khi tạo application.

## Test bắt buộc

- Tạo driver bằng tọa độ: lưu đúng, available mặc định false; update lại chỉ giữ tọa độ mới nhất.
- Availability trước tọa độ: chưa hiện nearby; sau khi có tọa độ thì hiện khi available=true.
- Thay đổi tọa độ không reset availability; thay đổi availability không mất tọa độ, kể cả hai thao tác đồng thời.
- Zero/biên tọa độ, thiếu dữ liệu ở mapping đích, invalid/NaN/Inf ở business input, false, radius không hợp lệ.
- Nearby: cùng vị trí khoảng cách 0; trong/ngoài bán kính; đúng ranh giới theo khoảng cách thực; driver unavailable hoặc thiếu tọa độ bị loại; sort và tie-break ổn định; mảng rỗng.
- Lưu/đọc qua hai application cùng DB, tạo lại application/repository không mất dữ liệu; lỗi DB và context timeout được trả đúng.
- Test PostgreSQL thật dưới build tag `integration`, dùng `TEST_LOCATION_DATABASE_URL` và ID riêng của test; thiếu DSN khi đã chọn tag phải báo lỗi, không âm thầm skip hoặc dùng RAM. Cleanup chỉ dữ liệu test sở hữu.

## Chạy và bàn giao

Tạo database test riêng một lần trên Compose infra đang chạy:

```bash
make infra-up
docker compose --profile infra exec -T postgres createdb -U ride location_test
```

Nếu database đã tồn tại và thuộc dev, dùng lại; không drop database của người khác. Integration test áp dụng SQL service rõ ràng trong setup, không cần sửa `make migrate`.

```bash
go test -race ./internal/location/...
TEST_LOCATION_DATABASE_URL='postgres://ride:ride_local@localhost:5432/location_test?sslmode=disable' \
  go test -race -tags=integration ./internal/location/... -count=1
make fmt-check vet test-race contract build
git diff --check
```

Khi chạy Go trực tiếp, dùng toolchain/cache theo môi trường dev; `.env` chỉ được Make tự load. Đổi host port DSN nếu dùng port khác. Điền [handoff Location](handoffs/location.md): constructor, repository interface, SQL, mapping API đích và bằng chứng test. Chưa tự bật entrypoint thật hoặc chỉnh fixture test chung.
