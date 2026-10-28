# RabbitMQ và event contract v1

Broker chạy thật; phiên bản hiện tại **chỉ dựng topology**, không có publisher/consumer hoặc bảo đảm đồng bộ event với database. 202 từ Matching/Notification là fixture. Cấu hình tại `infra/rabbitmq`; schema máy đọc tại `docs/events`.

| Exchange | Type | Routing key | Queue dự kiến / consumer |
| --- | --- | --- | --- |
| `rides.v1` | topic, durable | `ride.requested.v1` | `matching.requests.v1` / Matching |
| `rides.v1` | topic, durable | `ride.accepted.v1` | `notification.accepted.v1` / Notification |
| `rides.v1` | topic, durable | `ride.cancelled.v1` | `matching.cancelled.v1` / Matching |

Queue durable, không auto-delete. User local `ride` được tạo bởi image, definitions không chứa mật khẩu. URL `amqp://ride:ride_local@localhost:5672/` trong `.env` dành cho bước nối transport sau.

Envelope chung:

```json
{
  "eventId": "event_unique_id",
  "eventType": "ride.accepted.v1",
  "version": 1,
  "occurredAt": "2026-10-26T12:00:00Z",
  "requestId": "request_correlation_id",
  "data": {"rideId": "ride_123", "driverId": "driver_456"}
}
```

| Type / routing key | `data` bắt buộc |
| --- | --- |
| `ride.requested.v1` | `{rideId,riderId,fareId}` |
| `ride.accepted.v1` | `{rideId,driverId}` |
| `ride.cancelled.v1` | `{rideId,riderId}` |

`eventId` duy nhất cho event logic; retry publish giữ nguyên ID. `occurredAt` RFC3339 UTC; `version` luôn 1. Không đưa JWT, mật khẩu hoặc dữ liệu DB nội bộ vào payload. Đổi ý nghĩa hoặc shape phá vỡ tương thích phải thêm version/routing key mới.

Khi nối transport ở PR sau: dùng persistent messages, mandatory routing và publisher confirms; ACK thủ công sau khi xử lý thành công; consumer idempotent/deduplicate bằng eventId, xử lý redelivery và có retry hữu hạn/dead-letter. Không ACK rồi mới xử lý. Publisher confirm không chứng minh consumer đã xử lý.

Nếu cần bảo đảm trạng thái Ride commit luôn phát event, bổ sung transactional outbox cùng migration và kiểm thử crash trước/sau commit; không publish trực tiếp rồi tuyên bố atomic giữa PostgreSQL và RabbitMQ. Không thêm outbox ở phiên bản hiện tại vì transport chưa được nối.

Nguồn: [RabbitMQ Reliability](https://www.rabbitmq.com/docs/reliability), [Publisher Confirms](https://www.rabbitmq.com/docs/confirms).
