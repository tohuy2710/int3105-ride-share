# API catalog v1

Catalog này mô tả runtime hiện tại. [Phân công phát triển tiếp](team/README.md) và ba requirement trong bộ tài liệu team mô tả implementation thật đích; chúng chưa thay đổi OpenAPI/code đang chạy. Người tích hợp sẽ công bố contract/mapping mới trước khi bật các service thật.

OpenAPI trong `docs/openapi` là nguồn chuẩn đầy đủ. Tất cả API trả JSON, body tối đa 1 MiB; ID chuỗi không rỗng. `X-Request-ID` echo tối đa 128 ký tự hoặc sinh mới. Mọi lỗi có `{error:{code,message,details:[]}}`. `details` chỉ mô tả, client phân nhánh bằng `code`.

## Auth và Ride

| API | Input | Success | Quyền / hành vi |
| --- | --- | --- | --- |
| POST `/auth/register` | `{email,password,role}` | 201 `{userId:"user_mock",email:<input>,role:<input>,persisted:false}` | Public, không lưu user |
| POST `/auth/login` | `{email,password}` | 200 `{accessToken:<JWT>,tokenType:"Bearer",expiresIn:3600,user:{userId,email,role}}` | Public, chỉ fixture hợp lệ |
| GET `/auth/me` | — | 200 `{userId,email,role}` | JWT |
| POST `/fare` | `{pickupLocation:{lat,long},destination:{lat,long}}` | 200 `{fareId:"fare_123",estimatedFare:85000,etaMinutes:12}` | RIDER, mock |
| POST `/rides` | `{fareId}` | 201 `{rideId:<new>,status:"REQUESTED"}` | RIDER, lưu thật, không gọi Matching |
| GET `/rides/{rideId}` | path ID | 200 `{rideId,fareId,riderId,driverId,status}` | Rider sở hữu hoặc driver được gán |
| PATCH `/rides/{rideId}` | `{action:"ACCEPT"\|"DENY"}` | 200 `{rideId,action,driverId,status}` | DRIVER; ACCEPT thật, DENY đọc không lưu |
| PATCH `/rides/{rideId}/status` | `{status:"ARRIVED"\|"IN_PROGRESS"\|"COMPLETED"}` | 200 `{rideId,status}` | Driver đã nhận, chuyển tuần tự |
| DELETE `/rides/{rideId}` | path ID | 200 `{rideId,status:"CANCELLED"}` | Chủ cuốc; chỉ REQUESTED, retry CANCELLED được phép |

`driverId` là trường bắt buộc có trong snapshot/action, nullable. ACCEPT retry cùng driver chỉ idempotent khi status còn ACCEPTED. Status retry cùng trạng thái hiện tại trả 200. Sau COMPLETED không mở lại cuốc.

## Public API mock còn lại

| API | Input | Success | Quyền |
| --- | --- | --- | --- |
| POST `/drivers/location` | `{lat,long}` | 200 `{driverId:<JWT sub>,lat:<input>,long:<input>}` | DRIVER |
| PATCH `/drivers/availability` | `{available}` | 200 `{driverId:<JWT sub>,available:<input>}` | DRIVER |

Không lưu tọa độ hoặc availability; nearby không phụ thuộc hai endpoint này. Số 0 và boolean false hợp lệ; thiếu trường là 400.

## Contract cần cho service khác

Tất cả endpoint dưới đây yêu cầu `X-Internal-Token`. **Chưa có lời gọi tự động giữa các service**. Stub client được kiểm tra khớp fixture server; HTTP adapter chưa triển khai.

| Caller dự kiến → owner | Endpoint / input | Success xác định |
| --- | --- | --- |
| Matching → Ride | GET `/internal/rides/{rideId}` | 200 `{rideId,fareId,riderId,driverId,status}` đọc DB thật; 404 nếu thiếu |
| Matching → Location | GET `/internal/drivers/nearby?lat=&long=&radiusMeters=` | 200 `{drivers:[{driverId:"driver_456",location:{lat:21.0285,long:105.8542},distanceMeters:500}]}` |
| Ride → Matching | POST `/internal/matching/requests`, `{rideId}` | 202 `{rideId:<input>,status:"MATCHING"}` |
| Ride → Matching | GET `/internal/matching/requests/{rideId}` | 200 `{rideId:<path>,status:"MATCHING"}` |
| Matching → Notification | POST `/internal/notifications/ride-requests`, `{rideId,driverId,type:"RIDE_REQUEST"}` | 202 `{notificationId:"notification_123",rideId:<input>,driverId:<input>,type:"RIDE_REQUEST",status:"QUEUED"}` |

Nearby yêu cầu lat/long, radius mặc định 3000 và lớn hơn 0. 202 chỉ acknowledgement mock, không có job hoặc thông báo thật. RideStub trả fixture REQUESTED cho unit test; endpoint Ride internal thật đọc PostgreSQL. Matching/Location/Notification không xác minh ID tồn tại.

## Lỗi và endpoint vận hành

| HTTP | Code |
| --- | --- |
| 400 | `VALIDATION_ERROR` |
| 401 | `UNAUTHORIZED` |
| 403 | `FORBIDDEN` |
| 404 | `RIDE_NOT_FOUND` hoặc route `NOT_FOUND` |
| 405 | `METHOD_NOT_ALLOWED` |
| 409 | `ACCEPT_IN_PROGRESS`, `RIDE_ALREADY_ACCEPTED`, `INVALID_RIDE_STATE` |
| 503 | `DEPENDENCY_UNAVAILABLE` |
| 500 | `INTERNAL_ERROR` |

Gate bị giữ trả ACCEPT_IN_PROGRESS trước khi kiểm tra DB. Redis mất kết nối đi tiếp SQL; DB lỗi không trả thành công. Khi mất kết nối lúc commit, kết quả commit có thể chưa xác định: đọc lại hoặc retry cùng driver, không coi 503 là bằng chứng transaction đã rollback.

`/healthz` public trả `{service:"<name>-service",status:"ok"}`. `/readyz` public trả `{status:"ok"}` cho service stateless; Ride kiểm tra bảng DB, trả 503 unavailable hoặc 200 degraded khi chỉ Redis hỏng. Ride `GET /internal/metrics` trả counter lũy kế `{sqlAttempts,redisRejected,redisFallback}`; cần internal token, lấy delta và cộng các replica. `sqlAttempts` đếm lượt gọi transaction ACCEPT, không phải tổng số SQL statement.
