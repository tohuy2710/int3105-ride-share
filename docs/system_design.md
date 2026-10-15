# Uber System Design — API Endpoints by Service

Dựa trên tài liệu [HelloInterview – Design a Ride-Sharing Service Like Uber](https://www.hellointerview.com/learn/system-design/problem-breakdowns/uber), mình đề xuất các API endpoint cơ bản cho hệ thống Uber-like.

Phạm vi: Fare Estimation → Request Ride → Driver Matching → Driver Accept/Decline → Pickup → Complete Ride.

Mình giữ nguyên 4 service chính trong bài HelloInterview:

- Ride Service: Quản lý fare estimation và ride lifecycle.
- Location Service: Quản lý vị trí tài xế theo thời gian thực.
- Ride Matching Service: Tìm và ghép tài xế với chuyến đi.
- Notification Service: Gửi thông báo yêu cầu chuyến đi cho tài xế.

Phân biệt hai loại API:

- Source: Endpoint được HelloInterview định nghĩa trực tiếp.
- Proposed: Endpoint mình đề xuất thêm để hoàn thiện các luồng cơ bản, không phải endpoint có sẵn trong bài.

## 1. Ride Service

Service chịu trách nhiệm tính giá, tạo chuyến đi và quản lý trạng thái chuyến đi.

| Method    | Endpoint                   | Description                                                                       | Nguồn   |
| --------- | -------------------------- | --------------------------------------------------------------------------------- | -------- |
| `POST`  | `/fare`                  | Tính giá dự kiến và ETA                                                      | Source   |
| `POST`  | `/rides`                 | Rider xác nhận đặt xe                                                         | Source   |
| `GET`   | `/rides/{rideId}`        | Lấy thông tin và trạng thái chuyến đi                                      | Proposed |
| `PATCH` | `/rides/{rideId}`        | Driver chấp nhận hoặc từ chối chuyến đi                                    | Source   |
| `PATCH` | `/rides/{rideId}/status` | Cập nhật trạng thái chuyến đi: đến điểm đón, bắt đầu, hoàn thành | Proposed |

### Request mẫu

POST /fare

```
{
  "pickupLocation": {
    "lat": 21.0285,
    "long": 105.8542
  },
  "destination": {
    "lat": 21.035,
    "long": 105.821
  }
}
```

Response dự kiến:

```
{
  "fareId": "fare_123",
  "estimatedFare": 85000,
  "etaMinutes": 12
}
```

POST /rides

```
{
  "fareId": "fare_123"
}
```

Response dự kiến:

```
{
  "rideId": "ride_123",
  "status": "REQUESTED"
}
```

PATCH /rides/

```
{
  "action": "ACCEPT"
}
```

Hoặc:

```
{
  "action": "DENY"
}
```

Các endpoint chính ở trên bám theo phần API Design của HelloInterview. Response và tên trường JSON minh họa không phải schema chính thức của nguồn.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://www.hellointerview.com\&sz=32)

hellointerview\.com

## 2. Location Service

Quản lý vị trí hiện tại và thông tin availability của tài xế để phục vụ driver matching.

| Method    | Endpoint                     | Description                                                | Nguồn               |
| --------- | ---------------------------- | ---------------------------------------------------------- | -------------------- |
| `POST`  | `/drivers/location`        | Driver gửi tọa độ hiện tại                           | Source               |
| `PATCH` | `/drivers/availability`    | Driver bật/tắt trạng thái sẵn sàng nhận chuyến     | Proposed             |
| `GET`   | `/internal/drivers/nearby` | Matching Service truy vấn danh sách tài xế gần pickup | Proposed — Internal |

POST /drivers/location

```
{
  "lat": 21.0285,
  "long": 105.8542
}
```

PATCH /drivers/availability

```
{
  "available": true
}
```

Về implementation, Location Service có thể lưu tọa độ vào Redis Geospatial, và sử dụng proximity search để tìm tài xế ở gần. Đây là hướng được mô tả trong phần Location Deep Dive của HelloInterview.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://www.hellointerview.com\&sz=32)

hellointerview\.com

## 3. Ride Matching Service

Service tìm tài xế phù hợp dựa trên pickup location, vị trí hiện tại và availability.

| Method   | Endpoint                                 | Description                       | Nguồn               |
| -------- | ---------------------------------------- | --------------------------------- | -------------------- |
| `POST` | `/internal/matching/requests`          | Bắt đầu matching cho một ride | Proposed — Internal |
| `GET`  | `/internal/matching/requests/{rideId}` | Kiểm tra kết quả matching      | Proposed — Optional |

POST /internal/matching/requests

```
{
  "rideId": "ride_123"
}
```

Response dự kiến:

```
{
  "rideId": "ride_123",
  "status": "MATCHING"
}
```

Lưu ý: HelloInterview không định nghĩa HTTP endpoint cho Matching Service. Theo thiết kế trong bài, sau khi `POST /rides` tạo chuyến đi, Ride Service sẽ kích hoạt matching ở backend. Trong phương án sử dụng Kafka hoặc message queue, Matching Service có thể nhận event trực tiếp mà không cần `POST /internal/matching/requests`.

Vì vậy, hai endpoint này chỉ là lựa chọn nếu muốn giao tiếp giữa các service bằng HTTP.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://www.hellointerview.com\&sz=32)

hellointerview\.com

## 4. Notification Service

Gửi push notification đến driver khi có ride request cần phản hồi.

| Method   | Endpoint                                  | Description                                    | Nguồn               |
| -------- | ----------------------------------------- | ---------------------------------------------- | -------------------- |
| `POST` | `/internal/notifications/ride-requests` | Gửi thông báo chuyến đi mới đến driver | Proposed — Internal |

POST /internal/notifications/ride-requests

```
{
  "rideId": "ride_123",
  "driverId": "driver_456",
  "type": "RIDE_REQUEST"
}
```

Notification Service sau đó gửi thông báo tới Driver Client qua APNs hoặc FCM, đúng với mô hình trong HelloInterview.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://www.hellointerview.com\&sz=32)

hellointerview\.com

Endpoint HTTP này cũng là tùy chọn. Nếu Notification Service được kích hoạt bằng event/message thì không cần endpoint riêng.

## 5. API Flow tổng thể

CORE RIDE BOOKING FLOW

Rider Client

POST /fare → POST /rides

API Gateway

Ride Service

Create Ride → Trigger Matching

Ride Matching Service

Query Location Service → Select Driver

Notification Service

Push via APNs / FCM

Driver Client

PATCH /rides/{rideId} → ACCEPT / DENY

Ride Service

Update Ride Status

## 6. Tổng hợp endpoint nên có cho MVP

Nếu chỉ triển khai phần core requirements, mình đề xuất giữ bộ endpoint sau:

| Service      | Endpoint                                       | Priority                      |
| ------------ | ---------------------------------------------- | ----------------------------- |
| Ride         | `POST /fare`                                 | Required                      |
| Ride         | `POST /rides`                                | Required                      |
| Ride         | `GET /rides/{rideId}`                        | Required                      |
| Ride         | `PATCH /rides/{rideId}`                      | Required                      |
| Ride         | `PATCH /rides/{rideId}/status`               | Required                      |
| Location     | `POST /drivers/location`                     | Required                      |
| Location     | `PATCH /drivers/availability`                | Required                      |
| Location     | `GET /internal/drivers/nearby`               | Internal, tùy implementation |
| Matching     | `POST /internal/matching/requests`           | Optional                      |
| Notification | `POST /internal/notifications/ride-requests` | Optional                      |

Các quy ước nên thống nhất trong docs:

- `riderId` và `driverId` của người gọi được xác định thông qua session hoặc JWT, thay vì tin vào ID do client tự gửi.
- Giá chuyến đi được xác định từ `fareId` lưu ở backend.
- Client không gọi trực tiếp Matching Service hoặc Notification Service.
- Không cần tạo endpoint cho Kafka; Kafka dùng để truyền event giữa các service.

Bộ API này đủ để mô tả một hệ thống đặt xe cơ bản theo HelloInterview, không mở rộng sang payment, rating, scheduled rides hay những chức năng nằm ngoài phạm vi bài.
