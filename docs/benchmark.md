# Protocol benchmark acceptance

## Mục tiêu và hai chế độ

So sánh `sql` với `redis_sql` trên cùng cấu hình, code, DB pool, replica và fixture. Cả hai đều phải đúng dữ liệu. Redis chỉ giảm số lượt vào transaction ACCEPT; không mặc định kết luận cải thiện latency hoặc throughput.

`cmd/benchmark` là load generator Go qua HTTP, có barrier cho từng nhóm request. Suite Python khởi động PostgreSQL/Redis riêng, Auth và hai replica Ride. Một nhóm phân request xen kẽ vào hai replica bằng hai URL trực tiếp, không thêm gateway hoặc load balancer vào phép đo.

## Chạy

```bash
make build
python3 -m venv .cache/benchmark-venv
.cache/benchmark-venv/bin/python -m pip install -r scripts/benchmark-requirements.txt
.cache/benchmark-venv/bin/python scripts/benchmark_suite.py --output .cache/benchmark
```

Cần compiler và PostgreSQL/Redis native trên PATH. Suite mặc định dùng port PostgreSQL 15432, Redis 16379, Ride 18081/19081, Auth 18085. Có thể đổi port DB/Redis bằng `NATIVE_POSTGRES_PORT`, `NATIVE_REDIS_PORT`; port ứng dụng được dành riêng cho suite. Nếu port đã dùng, dừng và chọn cấu hình phù hợp; runner không dừng tiến trình của người khác.

Chạy nhanh để xác minh công cụ:

```bash
python3 scripts/benchmark_suite.py --concurrency 2,10 --groups 5 --warmup 1 --repeats 2 --no-plots
```

Đo từ hai replica tự chạy cùng mode:

```bash
RIDE_URLS=http://localhost:8081,http://localhost:9081 \
AUTH_URL=http://localhost:8085 ACCEPT_MODE=redis_sql \
make benchmark
```

Lệnh độc lập cần `.env` đúng DSN/internal token và fixture đủ tài xế. `-mode` là nhãn, không đổi mode của server đang chạy. Suite chịu trách nhiệm khởi động server đúng mode. Không chạy cạnh traffic khác vì metric lũy kế sẽ bị nhiễu.

## Workload và khoảng đo

- Concurrency mặc định `2,10,50,100`; 10 nhóm warm-up, 100 nhóm đo, 5 lần lặp mỗi case.
- `hot`: C tài xế khác nhau tranh một cuốc mới; đúng một người thắng.
- `disjoint`: C tài xế nhận C cuốc mới khác nhau; tất cả phải thắng.
- Mỗi nhóm đều tạo cuốc mới; không tái dùng cuốc đã ACCEPTED. Driver có thể dùng lại ở nhóm sau vì baseline không giới hạn cuốc/driver.
- Login, tạo cuốc, lấy metric và xác minh DB nằm ngoài khoảng đo. Khoảng đo tính từ mở barrier đến khi toàn bộ HTTP response về, gồm chi phí load generator và HTTP.
- Thời gian đo tổng là tổng thời gian các burst, không phải thời gian toàn bộ suite. Request throughput = attempts / thời gian đó; ride throughput = số cuốc có người thắng / thời gian đó. Đây là **burst throughput**, không phải phép đo steady-state hoặc open-loop arrival rate.
- Mỗi nhóm đối chiếu người thắng với PostgreSQL ngoài khoảng đo. Nhiều người thắng, thiếu người thắng, sai response hoặc lỗi ngoài contract làm command exit khác 0.

## Số liệu và artifacts

Mỗi mode có `*.json` và `*.csv`: từng request, group, driver, ride, status, error code và latency. JSON tổng hợp từng workload/concurrency/lần lặp:

- attempts, accepted, conflicts, errors và số cuốc thắng;
- request/ride throughput;
- p50/p95/p99 tách `accepted`, `conflict`, `error` và `all`;
- delta `sqlAttempts`, `redisRejected`, `redisFallback` cộng từ hai replica.

`sqlAttempts` là số lượt vào repository ACCEPT, không phải tất cả SQL statement. `409 ACCEPT_IN_PROGRESS` và `409 RIDE_ALREADY_ACCEPTED` là conflict dự kiến, không trộn với 5xx/transport error. Request bị từ chối nhanh không được gọi là cuốc nhận thành công.

`*-resources.csv` lấy mẫu 0.5 giây theo PostgreSQL, Redis, Auth, Ride và loadgen. Linux lấy CPU interval từ `/proc`; macOS dùng `%cpu` của `ps`. CPU 100% tương đương khoảng một core; tổng RSS có thể đếm lặp trang nhớ chia sẻ. Resource sampling gồm cả chuẩn bị nhóm/login, còn request latency chỉ đo trong burst.

`environment.json` ghi CPU, RAM, OS, tool version, commit/dirty state, cấu hình, fixture hash và binary SHA256. Archive source có manifest từng file. `comparison.png/.svg` dùng median và dải min/max giữa các lần lặp; không gán ý nghĩa confidence interval cho dải này. Không ghi secret, JWT hoặc mật khẩu vào artifacts.

## Tính công bằng và giới hạn

Suite chạy hai mode tuần tự trong cùng phiên, cùng PostgreSQL/Redis, chung cấu hình durability: `fsync=on`, `synchronous_commit=on`, shared_buffers 128 MB, max_connections 50, pool mặc định 10/replica. Mỗi case warm-up lại, dùng ID mới. Các mode có cùng số request và phân bố dữ liệu nhưng không dùng cùng ride ID.

Thứ tự mặc định SQL trước Redis+SQL có thể tạo hiệu ứng cache/order; cần ghi nhận điều này khi báo cáo. Dữ liệu tích lũy trong một run, ID mới bảo đảm không tranh dữ liệu cũ. Chỉ so sánh kết quả trong cùng môi trường; không so laptop với Kaggle như trước/sau cải tiến. Load generator và backend dùng chung CPU, loopback không phản ánh mạng production. RabbitMQ, Matching, Location và Notification không nằm trong đường đo.

Pha 2 có thể thêm workload steady-state, đảo thứ tự mode, soak test, cache kết quả hoặc chiến lược giảm contention mới. Không thêm phương án SQL sai dữ liệu để tạo baseline chậm/nhanh giả.
