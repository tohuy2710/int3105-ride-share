# API catalog v1.1

OpenAPI trong `docs/openapi` mô tả contract đầy đủ. JSON tối đa 1 MiB; unknown fields được bỏ qua. JWT/internal token vẫn bắt buộc. `X-Request-ID` được echo tối đa 128 ký tự hoặc sinh mới. Lỗi có `{error:{code,message,details:[]}}`.

## Luồng tạo cuốc → tìm tài xế → lưu thông báo

1. Driver đăng nhập, `POST /drivers/location` và `PATCH /drivers/availability` với `available:true` để được xét nearby.
2. Rider gọi `POST /rides` với `{fareId,pickupLocation:{lat,long},radiusMeters?:3000}`. Tọa độ đón bắt buộc; 0 hợp lệ; radius phải dương, không nhận null. Ride ghi cuốc REQUESTED và outbox matching trong cùng transaction. 201 xác nhận hai bản ghi đã commit, chưa xác nhận matching/thông báo đã xong.
3. Dispatcher Ride poll mỗi giây, claim một row bằng lease SQL 15 giây rồi gọi Matching ngoài transaction. Nhiều replica dùng SKIP LOCKED; sau crash, replica khác retry khi lease hết. Downstream timeout/lỗi/mất response giữ row để retry. Cuốc đã được nhận hoặc hủy trước dispatch thì row được hoàn tất mà không gửi thêm yêu cầu.
4. Matching đọc trạng thái cuốc qua HTTP Ride, lấy tài xế available có tọa độ trong radius qua Location. Job lưu pickup/radius và snapshot ứng viên. Không có ứng viên trả `NO_DRIVERS`, không tự tìm lại khi driver online sau đó.
5. Với mỗi ứng viên, Matching gọi Notification để lưu `RIDE_REQUEST`. Retry cùng tuple không tạo row mới. Mất acknowledgement có thể gửi lại nhưng unique constraint giữ một notification. Dispatcher chỉ đánh dấu hoàn tất sau response matching hợp lệ.
6. Driver gọi ACCEPT trên Ride như trước. Matching không gán người thắng. GET Matching đối chiếu Ride để chuyển `MATCHED` hoặc `CANCELLED`; terminal status không lùi lại. Notification đã QUEUED không bị thu hồi khi cuốc đổi trạng thái; có thể có notification đang gửi trong lúc hủy/nhận cuốc.

Không có push/device delivery, RabbitMQ publisher/consumer hoặc event phát khi ACCEPT. `QUEUED` chỉ là dữ liệu đã lưu. Cơ chế retry là at-least-once với dedup ở service đích. POST tạo cuốc mới mỗi lần; chưa có idempotency key cho thao tác tạo cuốc.

## Auth và Ride

| API | Input | Success | Hành vi |
| --- | --- | --- | --- |
| POST `/auth/register` | `{email,password,role}` | 201 `persisted:false` | Mock, không lưu tài khoản |
| POST `/auth/login` | `{email,password}` | 200 JWT/user | bcrypt fixture, JWT thật |
| GET `/auth/me` | — | 200 user | JWT |
| POST `/fare` | `{pickupLocation,destination}` | 200 `{fareId:"fare_123",estimatedFare:85000,etaMinutes:12}` | RIDER; giá cố định |
| POST `/rides` | `{fareId,pickupLocation,radiusMeters?}` | 201 `{rideId,status:"REQUESTED"}` | RIDER; lưu cuốc + outbox thật |
| GET `/rides/{rideId}` | ID | 200 snapshot | Chủ cuốc hoặc driver được gán |
| PATCH `/rides/{rideId}` | `{action:"ACCEPT"\|"DENY"}` | 200 action snapshot | DRIVER; ACCEPT SQL + Redis gate; DENY chỉ đọc |
| PATCH `/rides/{rideId}/status` | `{status:"ARRIVED"\|"IN_PROGRESS"\|"COMPLETED"}` | 200 ride/status | Driver đã nhận; chuyển tuần tự |
| DELETE `/rides/{rideId}` | ID | 200 CANCELLED | Chủ cuốc; chỉ REQUESTED, retry CANCELLED được phép |

`driverId` bắt buộc trong snapshot, nullable. Retry ACCEPT cùng driver chỉ idempotent khi còn ACCEPTED. Retry trạng thái hiện tại trả 200. Không mở lại cuốc COMPLETED. SQL là quyết định cuối cùng khi Redis hết TTL hoặc mất kết nối.

## Location

| API | Input | Success | Quyền |
| --- | --- | --- | --- |
| POST `/drivers/location` | `{lat,long}` | 200 `{driverId,lat,long}` | DRIVER |
| PATCH `/drivers/availability` | `{available}` | 200 `{driverId,available}` | DRIVER |
| GET `/internal/drivers/nearby` | query `lat,long,radiusMeters?` | 200 `{drivers:[{driverId,location,distanceMeters}]}` | Internal token |

Lưu tọa độ và availability độc lập, không ghi đè cột còn lại. Chỉ driver available và có tọa độ mới được xét. Haversine, sắp gần trước rồi theo driverId; không có TTL nên driver phải tự tắt availability. Radius mặc định 3000, số 0/false hợp lệ ở tọa độ/availability.

## Internal service API

Tất cả endpoint dưới đây yêu cầu `X-Internal-Token`; runtime dùng HTTP client có timeout và không theo redirect.

| Caller → owner | Endpoint / input | Success |
| --- | --- | --- |
| Matching → Ride | GET `/internal/rides/{rideId}` | 200 snapshot đọc DB |
| Ride → Matching | POST `/internal/matching/requests`, `{rideId,pickupLocation,radiusMeters?}` | 202 `{rideId,status}` |
| Operator → Matching | GET `/internal/matching/requests/{rideId}` | 200 `{rideId,status}` |
| Matching → Notification | POST `/internal/notifications/ride-requests`, `{rideId,driverId,type:"RIDE_REQUEST"}` | 202 `{notificationId,rideId,driverId,type,status:"QUEUED"}` |
| Operator → Notification | GET `/internal/notifications/{notificationId}` | 200 cùng snapshot |

Matching status: `MATCHING`, `NO_DRIVERS`, `MATCHED`, `CANCELLED`. Retry cùng ride/pickup/radius dùng cùng job; input khác trả 409 `MATCHING_REQUEST_CONFLICT`. Lần đầu chỉ nhận cuốc REQUESTED. POST Matching 202 chỉ được trả sau khi mọi yêu cầu notification trong snapshot được lưu; job có thể đã lưu trong khi POST trả 503, nên GET thấy job không có nghĩa dispatch notification đã hoàn tất.

## Lỗi và vận hành

| HTTP | Code |
| --- | --- |
| 400 | `VALIDATION_ERROR` |
| 401 | `UNAUTHORIZED` |
| 403 | `FORBIDDEN` |
| 404 | `RIDE_NOT_FOUND`, `MATCHING_NOT_FOUND`, `NOTIFICATION_NOT_FOUND`, `NOT_FOUND` |
| 405 | `METHOD_NOT_ALLOWED` |
| 409 | `ACCEPT_IN_PROGRESS`, `RIDE_ALREADY_ACCEPTED`, `INVALID_RIDE_STATE`, `MATCHING_REQUEST_CONFLICT` |
| 503 | `DEPENDENCY_UNAVAILABLE` |
| 500 | `INTERNAL_ERROR` |

DB/commit timeout không trả acknowledgement thành công giả. Kết quả commit có thể chưa xác định nếu mất kết nối lúc commit; retry cùng tuple notification/matching là an toàn, nhưng POST tạo cuốc chưa có idempotency key.

Mỗi service có `/healthz`, `/readyz`, `/swagger/index.html`, `/swagger/openapi.json`, `/swagger/openapi.yaml`. Ride/Location/Matching/Notification readiness kiểm tra bảng DB của mình; không tự migrate. Ride Redis lỗi trả 200 degraded, DB lỗi trả 503. Auth stateless. `GET /internal/metrics` trên Ride cần internal token, trả `{sqlAttempts,redisRejected,redisFallback}`; lấy delta và cộng các replica. `sqlAttempts` đếm transaction ACCEPT, không phải mọi SQL statement.
