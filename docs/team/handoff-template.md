# Mẫu bàn giao service

Điền trực tiếp vào file được giao trong `handoffs/`, không sửa mẫu này trong PR service. Không ghi mật khẩu/JWT/secret/log dữ liệu riêng vào Git.

## Trạng thái và Git

- Service / người phụ trách:
- Trạng thái: NOT_STARTED / IN_PROGRESS / READY_FOR_REVIEW / INTEGRATED.
- BASE_SHA: commit chung bắt đầu công việc, gồm bộ instruction.
- HEAD_SHA: commit được bàn giao; nếu chưa commit ghi rõ working tree và commit SHA sau khi commit, không tự đoán SHA.
- Branch / PR URL nếu đã push hoặc mở PR:
- Chỉ thay đổi trong ownership: có/không; danh sách file `git diff --name-only BASE_SHA...HEAD` hoặc diff working tree tương ứng.

## Đã triển khai và chưa triển khai

- Nghiệp vụ và kịch bản đã hoàn thành:
- Các file/constructor/application/repository đã thêm:
- Mock và endpoint/default entrypoint baseline còn nguyên:
- Phần chưa làm hoặc giới hạn (nhắc lại không có transport/default wiring/delivery nếu thuộc phạm vi):

## Cách người tích hợp khởi tạo

Ghi chữ ký constructor, model/port và ví dụ Go ngắn dùng implementation thật; chỉ rõ file để đối chiếu. Với Matching, ghi fake đã dùng trong test và adapter port nào còn thiếu. Không yêu cầu người tích hợp tự đoán cấu hình/khởi tạo.

## Database và cấu hình

- SQL file, schema/table/constraint được tạo:
- Cách áp dụng rõ ràng vào database test riêng; thứ tự SQL nếu nhiều file:
- Integration test DSN env name (không ghi secret thực tế):
- Cấu hình runtime đề xuất; giá trị mặc định, dependency mới nếu thật sự cần:
- Lý do không ảnh hưởng bảng/transaction/key Redis của Ride:

## Delta contract cần máy tích hợp công bố

| Endpoint/port/model | Hiện tại | Đích | Tác động caller/test và mapping cần làm |
| --- | --- | --- | --- |
| Điền phần của service | | | |

Nêu rõ input bắt buộc mới, enum/mã lỗi mới, idempotency, status code, nullable field. Không sửa shared OpenAPI/contracts/error mapper trong PR service. Nếu không đổi shape, vẫn nêu thay đổi từ fixture sang dữ liệu thật.

## Bằng chứng kiểm thử

| Lệnh/kịch bản | Đã chạy? | Kết quả thực tế | Môi trường/giới hạn |
| --- | --- | --- | --- |
| Unit/race service | | | |
| PostgreSQL integration của service | | | |
| make fmt-check vet test-race contract build | | | |
| git diff --check và kiểm tra phạm vi | | | |

Không đánh dấu pass cho test bị skip, chưa có DSN hoặc chỉ dùng RAM. Nếu môi trường thiếu dependency, ghi lỗi và phần đã kiểm chứng, không sửa test cho qua. Không ghi số liệu local như số liệu Kaggle.

## Checklist trước khi gửi review

- [ ] Code chỉ nằm trong service được giao và file handoff của mình.
- [ ] Không đổi MockService/interface/router/handler/DTO hiện tại hoặc mặc định entrypoint.
- [ ] Không sửa Ride/Auth/platform/contracts/clients, shared tests, root migration, môi trường, dependency hoặc benchmark.
- [ ] SQL chỉ thuộc schema service; không có FK/query xuyên service, startup migration hoặc outbox.
- [ ] PostgreSQL thật đã kiểm tra persistence/concurrency; các lỗi/giới hạn được ghi rõ.
- [ ] Người tích hợp có đủ constructor, SQL, contract delta và lệnh test để tiếp nhận.
- [ ] Không commit .env/cache/binary/log/JWT; không tự merge/push main.

## Dành cho người tích hợp

- Commit đã review/merge và thứ tự:
- Contract/migration/wiring đã áp dụng:
- Regression/race/fault/smoke/benchmark thực tế đã chạy:
- Mốc kết luận: application/repository accepted / endpoint real integrated / transport integrated.
- Phần còn lại:
