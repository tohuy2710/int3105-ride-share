# Người 2 — yêu cầu Matching thật

Trạng thái: **đặc tả đích**, khác input/mock đang chạy. Đọc [phân công](README.md), [prompt](matching-agent.md), [OpenAPI hiện tại](../openapi/matching.yaml) và [catalog](../api-catalog.md).

## Mục tiêu và giới hạn

Thêm application và PostgreSQL repository trong `internal/matching/`. Matching tìm một tập tài xế ứng viên và yêu cầu lưu thông báo, **không quyết định tài xế thắng**. Chỉ API ACCEPT của Ride được gán tài xế bằng baseline PostgreSQL/Redis.

Không sửa Ride, thêm pickup vào Ride/fare, truy cập bảng Ride hoặc tự gọi ACCEPT. Không biến trạng thái Matching thành nguồn chuẩn về assignment. Giữ interface/mock/HTTP đang chạy; input nghiệp vụ mới có thể là model nội bộ của service để không phải sửa `contracts.MatchingRequest` trước khi người tích hợp công bố thay đổi.

## Input/output đích

POST `/internal/matching/requests` khi được người tích hợp bật thật:

```json
{
  "rideId": "ride_123",
  "pickupLocation": {"lat": 21.0285, "long": 105.8542},
  "radiusMeters": 3000
}
```

- Pickup bắt buộc ở contract đích vì Ride hiện không lưu tọa độ. Không suy ra từ fare_123, không dùng tọa độ fixture hoặc (0,0) thay cho trường thiếu. Tọa độ (0,0) được gửi rõ ràng là hợp lệ.
- Lat/long cùng giới hạn Location. Radius thiếu được boundary chuẩn hóa thành 3000, cung cấp thì phải hữu hạn và >0; application nhận radius đã chuẩn hóa.
- Đây là thay đổi không tương thích với caller chỉ gửi rideId ở runtime thật. Người tích hợp phải công bố OpenAPI/DTO/client, cập nhật caller và contract/smoke tests trước khi bật. Lần bàn giao này không sửa endpoint đang mock để âm thầm chấp nhận hoặc từ chối theo luật mới.
- Thành công POST trả `202 {rideId,status}`; GET `/internal/matching/requests/{rideId}` trả `200 {rideId,status}`. Cả hai dùng internal token ở HTTP boundary.

| Status đích | Ý nghĩa |
| --- | --- |
| MATCHING | Có ứng viên; đang chờ tài xế nhận, có thể còn yêu cầu thông báo cần retry |
| NO_DRIVERS | Tìm lần đầu không có tài xế phù hợp |
| MATCHED | Port đọc Ride quan sát ACCEPTED/ARRIVED/IN_PROGRESS/COMPLETED |
| CANCELLED | Port đọc Ride quan sát CANCELLED |

Không thêm status vào contract đang công bố trong PR riêng. Mock tiếp tục chỉ trả MATCHING.

## Port nghiệp vụ

Định nghĩa port nội bộ, nhận context và kiểu dữ liệu thuần. Không viết HTTP/AMQP adapter liên service:

| Port | Dữ liệu cần |
| --- | --- |
| RideReader | Snapshot `rideId,fareId,riderId,driverId,status` như `contracts.Ride` |
| DriverFinder | Nhận pickup/radius, trả danh sách như `contracts.NearbyResponse` |
| NotificationRequester | Nhận rideId/driverId/type=RIDE_REQUEST, trả snapshot acknowledgement như `contracts.NotificationResponse` |
| MatchingRepository | Tạo/đọc job theo rideId, giữ input và tập ứng viên, cập nhật tiến độ/status an toàn |

Người tích hợp map ba port đầu vào HTTP/client chung sau. Dev dùng fake port có thể điều khiển response/lỗi trong test; không đọc DB Location/Notification để thay thế lời gọi qua port.

## Luồng nghiệp vụ

1. Validate input; tìm job hiện có theo rideId. Nếu đã có nhưng pickup/radius khác giá trị đã chuẩn hóa ban đầu, trả conflict, không overwrite input hoặc ứng viên.
2. Đọc snapshot Ride qua port. Không có Ride trả lỗi not found; dependency lỗi trả unavailable, không giả thành NO_DRIVERS. Job mới chỉ được tạo nếu Ride REQUESTED.
3. Với job mới, gọi DriverFinder một lần cho pickup/radius. Deduplicate driverId, giữ thứ tự khoảng cách/ID từ kết quả Location. Lưu input và danh sách ID ứng viên cố định; không gán driverId của Ride.
4. Danh sách rỗng: lưu NO_DRIVERS và trả acknowledgement. Retry cùng input không tự tìm lại khi availability thay đổi; tìm lại/re-match chưa thuộc phạm vi.
5. Có ứng viên: lưu MATCHING trước side effect thông báo; gọi NotificationRequester cho từng ứng viên với đúng rideId/driverId/type. Chỉ trả 202 khi tất cả acknowledgement đã nhận. Không cần worker nền hoặc fanout tối ưu ở bản tối thiểu.
6. Nếu thông báo lỗi một phần: giữ job và danh sách đã chọn, trả 503. Retry cùng input đọc lại job và gửi lại các bộ ba từ danh sách cố định; Notification thật phải deduplicate. Không tuyên bố exactly-once hoặc rollback được notification đã ghi ở service khác.
7. Retry khi Ride vẫn REQUESTED dùng job cũ; không query nearby để thay tập ứng viên. Job NO_DRIVERS vẫn NO_DRIVERS. Retry MATCHING có thể gọi lại thông báo idempotent, không tạo job mới.
8. Nếu job đã có và Ride đã nhận/hủy: cập nhật thành MATCHED/CANCELLED, trả job hiện có khi retry POST hoặc GET; không phát thêm yêu cầu thông báo. GET không dispatch hoặc retry notification.
9. Hai request tạo job đồng thời phải hợp nhất thành một row/input/tập ứng viên bằng repository transaction/unique constraint. Chỉ tập ứng viên đã commit mới được dùng để gửi; tiến trình thua lượt tạo đọc lại job đó, không gửi từ danh sách tính tạm riêng của mình. Nếu input khác thì conflict.

Nếu trạng thái Ride đổi sau snapshot nhưng trước khi dispatch, một thông báo đến muộn vẫn có thể được lưu. Chấp nhận giới hạn này trong baseline; không dùng nó làm lý do sửa transaction ACCEPT hoặc thêm distributed transaction. Tài xế nhận cuốc vẫn phải đi qua Ride để kiểm tra trạng thái hiện tại.

## Dữ liệu và lỗi

SQL đề xuất `internal/matching/postgres/migrations/001_init.sql`, schema `matching`, bảng `requests`: ride_id PK, pickup_lat/long, radius_meters, status, candidate_driver_ids JSONB array, created_at, updated_at. Input không null, ID không rỗng, radius >0, trạng thái theo enum đích, candidate list không null. Có thể lưu tiến độ hoàn tất dispatch trong cùng row để phân biệt retry còn dở; không tạo bảng ngoài nhu cầu này. Không có FK sang Ride/driver/Notification.

Repository cập nhật status không làm mất input/danh sách cố định; MATCHED/CANCELLED không được ghi lùi thành MATCHING bởi retry cũ. Không giữ transaction DB mở trong lúc gọi port bên ngoài. Không cần bảng outbox vì chưa có transport đảm bảo event delivery.

| Tình huống | Kết quả đích |
| --- | --- |
| Input sai | 400 VALIDATION_ERROR |
| GET chưa có job | 404 MATCHING_NOT_FOUND — mới |
| RideReader xác nhận ride không tồn tại | 404 RIDE_NOT_FOUND |
| Job mới nhưng Ride không REQUESTED | 409 RIDE_NOT_MATCHABLE — mới |
| Cùng ride nhưng input khác | 409 MATCHING_REQUEST_CONFLICT — mới |
| DB/RideReader/DriverFinder/NotificationRequester lỗi hoặc timeout | 503 DEPENDENCY_UNAVAILABLE |

Lỗi domain không chứa HTTP/Gin; ghi mapping đề xuất vào handoff, không tự sửa global error mapper hoặc trả fake success. Nếu RideReader lỗi khi GET job đã có, trả unavailable, giữ trạng thái lưu hiện tại và không đoán kết quả Ride.

## Test và bàn giao

Unit/application test dùng fake ports; PostgreSQL integration dùng repository thật và fake remote ports:

- Một ứng viên, nhiều ứng viên và không có ứng viên; đúng pickup/radius được truyền đi; ứng viên trùng ID không tạo thông báo trùng trong cùng lượt dispatch.
- Ride thiếu, đã nhận, đã hủy; input thiếu/sai ở boundary đề xuất; không có bất kỳ port/method nào để ghi assignment hoặc gọi ACCEPT.
- Retry input giống/khác; NO_DRIVERS không tự re-match; partial Notification failure rồi retry giữ nguyên tập ứng viên và bộ ba yêu cầu.
- Hai application cùng DB tạo đồng thời chỉ một job; không dispatch danh sách chưa commit; input khác conflict; terminal status không bị overwrite bởi request cũ.
- GET phản ánh status từ Ride, không dispatch; dependency lỗi không được đổi thành NO_DRIVERS/MATCHED.
- Tạo lại application/repository vẫn đọc và retry được job. Không giữ transaction khi fake remote port đang chờ; context cancellation được truyền qua port.

```bash
make infra-up
docker compose --profile infra exec -T postgres createdb -U ride matching_test
go test -race ./internal/matching/...
TEST_MATCHING_DATABASE_URL='postgres://ride:ride_local@localhost:5432/matching_test?sslmode=disable' \
  go test -race -tags=integration ./internal/matching/... -count=1
make fmt-check vet test-race contract build
git diff --check
```

Tạo test database một lần, không drop DB khác. Integration setup áp dụng SQL riêng rõ ràng; tag integration thiếu DSN phải báo lỗi, không skip/fallback RAM. Go trực tiếp không tự load `.env`. Điền [handoff Matching](handoffs/matching.md), gồm constructor/port, SQL, field/status/error mới và test. Không cần chờ endpoint thật của hai người kia để hoàn tất mốc application/repository.
