# Công việc máy hiện tại — tích hợp cuối

Máy này giữ quyền phát triển Ride/race, Auth và các phần dùng chung đã có; ba dev bàn giao service theo [phân công](README.md). Quy trình dưới đây dành cho lúc tiếp nhận code, **chưa thực hiện merge, migration, thay contract hoặc nối transport trong lần viết tài liệu này**.

## 1. Giữ baseline và publish điểm xuất phát

Baseline race: `8dd4bd0cadefe73c82b92ee5fd06c87b06cf7cd9`. Publish commit tài liệu chung, thông báo SHA để ba người tạo branch/worktree có instruction. Không buộc họ checkout baseline cũ rồi mất tài liệu mới. Không commit/push thay họ hoặc tự tạo PR trong công việc chỉ viết tài liệu.

Giữ bản kiểm thử `sql`/`redis_sql`, hai replica, Redis fallback/lease/token release, transaction ACCEPT, ownership, retry, cancellation/status, mock contract và workload/metric benchmark. Bất kỳ phát triển Ride/Auth trên máy này là PR/công việc riêng, có mô tả và regression tương ứng; không gộp thay đổi đó vào patch của dev khác.

## 2. Review trước merge

Yêu cầu branch/HEAD SHA và handoff hoàn chỉnh. Sau khi fetch branch từ remote đã thống nhất, kiểm tra patch so với BASE_SHA thực tế của lần giao việc:

```bash
git diff --name-status BASE_SHA...SERVICE_HEAD
git diff --check BASE_SHA...SERVICE_HEAD
git diff BASE_SHA...SERVICE_HEAD -- cmd internal/ride internal/auth internal/platform internal/contracts internal/clients migrations infra scripts notebooks fixtures .github
git diff BASE_SHA...SERVICE_HEAD -- internal/integration internal/contracttest docs/openapi compose.yaml Dockerfile .env.example .go-version go.mod go.sum Makefile
```

Thay placeholder bằng SHA đã xác minh, không chạy như tên branch giả. Hai diff vùng bảo vệ phải rỗng đối với patch service. Tất cả path còn lại cũng phải nằm trong service được giao và handoff của họ; chú ý cả file bị xóa/đổi tên, không chỉ dòng thêm. Root/shared Markdown cũng không thuộc PR service.

Nếu có thay đổi ngoài phạm vi: yêu cầu tách/revert phần đó khỏi branch service hoặc tiếp nhận bằng PR tích hợp riêng. Không dùng `git checkout --ours/--theirs` cho cả thư mục, không cherry-pick rồi bỏ qua conflict không hiểu nguyên nhân và không đổi expected test để che regression. Khi rebase, kiểm tra lại patch trên base mới; không nhầm thay đổi của người tích hợp đã có trên base với thay đổi của dev.

## 3. Thứ tự tiếp nhận và migration

Tiếp nhận **Location → Notification → Matching**, mỗi lần chạy lại check trước khi tiếp nhận service kế tiếp. Matching vẫn có thể review application trước bằng fake ports.

- Review SQL schema service, constraint và test PostgreSQL thật; không có FK/read/write xuyên service hoặc ghi vào `ride.rides`.
- Trước migration mới đầu tiên, bổ sung cơ chế migration version/checksum trên máy này; giữ nguyên SQL bootstrap Ride hiện tại. Theo dõi version ở metadata migration hạ tầng riêng, không trộn vào model Ride. Chạy migration bằng command rõ ràng trước startup, không từ từng replica.
- Áp dụng SQL theo schema location/notification/matching sau bootstrap; không sửa migration đã áp dụng để nâng cấp database cũ. Không cho phép migration service làm DROP/ALTER schema Ride hoặc reset volume.
- Test trước trên database test/disposable của người tích hợp, rồi chạy lại trên database mới từ bootstrap + migration mới để phát hiện thiếu thứ tự. Không áp dụng SQL branch chưa review vào DB đang dùng.

## 4. Công bố contract đích và nối application

Thực hiện có chủ ý ở máy này, sau khi hiểu constructor/port/SQL và trước khi bật runtime thật:

| Service | Phần phải cập nhật cùng nhau |
| --- | --- |
| Location | Dữ liệu thật thay fixture, tập kết quả rỗng/sort/radius; constructor/repository/handler và readiness cần DB |
| Notification | ID thật, retry trả cùng ID, GET internal mới, NOTIFICATION_NOT_FOUND, client port đọc nếu cần và readiness DB |
| Matching | pickupLocation bắt buộc ở runtime thật, radius mặc định 3000, enum MATCHING/NO_DRIVERS/MATCHED/CANCELLED, lỗi mới và các port nghiệp vụ |

Cập nhật OpenAPI, catalog, `internal/contracts`, client interface/stub liên quan, global error mapper, handler mapping, `cmd/<service>` và cấu hình Docker/test đồng bộ. Mock implementation cùng bộ test fixture cũ được giữ để kiểm tra tương thích scaffold; thêm test runtime thật thay vì sửa fixture cũ thành dữ liệu tùy ý.

Contract Matching đích không tương thích caller chỉ gửi rideId. Công bố version/changelog và chuyển tất cả caller/test/smoke sang truyền pickup/radius trước khi bật, không dùng fallback tọa độ giả. Chưa nối tự động từ Ride khi chưa có nguồn pickup đúng; ở đợt đầu có thể gọi endpoint Matching đích trực tiếp với input rõ ràng. Không thêm pickup vào Ride hoặc sửa ACCEPT chỉ để làm Matching chạy được.

Mốc `endpoint real integrated` cần handler/application/repository thật, DB readiness, contract/request/response/error/security tests và smoke dữ liệu thật đã qua. Một PR chỉ thêm application không được đánh dấu là endpoint đã bật.

## 5. Nối HTTP/RabbitMQ — công việc riêng ở máy này

- Map RideReader → GET internal Ride; DriverFinder → nearby Location; NotificationRequester → POST Notification, có internal token/context/timeout/error mapping. Không query trực tiếp DB service khác. Không dùng fake port khi dependency thật lỗi.
- Chọn một đường kích hoạt matching/thông báo cho từng hành vi; không đồng thời bật HTTP và event cho cùng tác vụ rồi vô tình tạo dispatch hai lần. Ghi rõ đường đã bật trong tài liệu và test; idempotency vẫn cần thiết vì retry/redelivery.
- RabbitMQ hiện chỉ có topology. Trước khi nối publisher/consumer, công bố đầy đủ schema payload, routing, retry, ACK và dedup. Event `ride.requested.v1` hiện chưa có pickup: không giả định event đủ để chạy Matching thật; giữ đường này chưa bật đến khi đã có contract/nguồn pickup được review.
- Consumer ACK sau khi xử lý thành công; publish persistent/mandatory với confirms và xử lý trùng theo eventId. Không tuyên bố exactly-once.
- Không publish đồng bộ hoặc chờ remote HTTP trong transaction ACCEPT. Nếu cần bảo đảm commit Ride và phát event, thiết kế outbox trong PR Ride riêng, không thêm như dependency ngầm của ba service.

Mốc `transport integrated` chỉ đóng khi đã có đường dữ liệu thực và kiểm thử lỗi/duplicate tương ứng. Các mục này không tự cấp quyền thay đổi baseline race trong PR service.

## 6. Kiểm thử sau mỗi lần ghép

1. Chạy `make fmt-check vet test-race contract build` và PostgreSQL integration của service theo handoff.
2. Chạy `make integration` với TEST_DATABASE_URL/TEST_REDIS_URL trỏ dependency test, hoặc `python3 scripts/native_check.py` cho owned native dependencies. Không thay DSN về dữ liệu đang dùng để tiện chạy.
3. Kiểm tra vùng Ride so với baseline và phần phát triển riêng đã được review. Với một merge service thuần, patch vùng bảo vệ phải rỗng.
4. Chạy Compose/smoke; khi bật service thật, sửa smoke trên máy này theo dữ liệu động và vẫn giữ kịch bản auth, ACCEPT, retry, cancel/status. Không bắt nearby/notification thật trả fixture ID cũ để làm smoke qua.
5. Trước khi chốt bản cuối, chạy benchmark ngắn hai mode, hai replica, chẳng hạn `python3 scripts/benchmark_suite.py --concurrency 2,10,50,100 --groups 10 --warmup 2 --repeats 2 --no-plots`. Đây là kiểm tra regression, không thay đánh giá Kaggle.
6. Xác nhận một cuốc một người thắng, Redis lỗi vẫn đúng dữ liệu, gate bị giữ không vào SQL, TTL/kill-holder không làm sai DB, ACCEPT/DELETE chỉ một mutation thắng và transaction chỉ thành công sau commit. Giữ cấu hình/mẫu tải/metric baseline; biến động hiệu năng phải được báo cáo, không sửa workload cho đẹp số.

Nếu regression race xuất hiện, chưa tiếp nhận service kế tiếp hoặc bật runtime thật; giữ lại bản baseline dùng được và sửa trên nhánh tích hợp. Không xóa/skip test. Nếu môi trường không cho chạy một check, ghi rõ chưa kiểm chứng; không đánh dấu mốc hoàn tất thay cho kết quả.

## 7. Chốt bàn giao cuối

Điền mục người tích hợp trong từng handoff: SHA đã ghép, migration/contract/wiring, test thật đã chạy và mốc được đóng. Cập nhật README/catalog/development để phản ánh implementation đã bật, giữ rõ phần chưa có provider/transport. Publish commit cuối sau khi review; không force-push nhánh người khác.

Kaggle vẫn cần Save & Run All trên tài khoản thật. Service mới không được tự động đưa vào đường đo race; benchmark acceptance phải giữ môi trường và cách đo đã công bố. Tài liệu phân công hoặc test local không phải bằng chứng Kaggle.
