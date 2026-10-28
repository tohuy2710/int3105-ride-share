# Chạy trên Kaggle CPU

## Chuẩn bị source

Dùng [notebooks/kaggle_cpu.ipynb](../notebooks/kaggle_cpu.ipynb), chọn Accelerator **None**, bật Internet. Kaggle có thể thay đổi image/CPU giữa các phiên, vì vậy giữ notebook version, log môi trường và đo hai mode trong cùng phiên. Không giả định luôn có một số core/RAM cố định. [Kaggle Notebooks](https://www.kaggle.com/docs/notebooks).

Hai cách cung cấp code:

1. Push implementation lên GitHub, đặt `REVISION` trong notebook thành commit SHA chứa thay đổi.
2. Chạy `python3 scripts/package_kaggle.py`, tạo `.cache/kaggle-source.tar.gz`; attach archive dưới dạng Kaggle Dataset rồi đặt `SOURCE_ARCHIVE` theo đường dẫn `/kaggle/input/...`. Archive chứa source hiện tại kể cả thay đổi chưa commit, kèm manifest SHA256; không chứa `.env`, cache, `.git` hoặc work notes.

Notebook clone/extract vào `/kaggle/working/int3105-ride-share`. Dùng phiên mới khi đổi source archive; không chồng file lên một phiên đã chạy. Mặc định GitHub `main` chỉ dùng được sau khi implementation đã được push; source trong workspace hiện tại không tự xuất hiện trên GitHub.

## Cách chạy native

`scripts/kaggle_bootstrap.py` kiểm tra Linux amd64, compiler `gcc`, `make`, `perl`, và `runuser` khi notebook chạy root. Script tải Go 1.25.9, PostgreSQL 16.10 và Redis 7.4.2 từ nguồn chính thức, kiểm SHA256 trong `infra/versions.json`, build vào `/kaggle/working/ride-share-tools`. Không cài apt, không cần Docker daemon/systemd. Log compile nằm trong tools directory.

PostgreSQL compile không readline/zlib/ICU để giảm dependency môi trường; Redis dùng allocator libc. Cả hai mode dùng cùng binary và cấu hình này, không so native build với Docker để kết luận hiệu năng.

Khi notebook chạy root, database chạy dưới `nobody`; thư mục run cho phép traverse, data directory thuộc user đó. PostgreSQL không cho chạy server/initdb bằng root. [Tài liệu initdb](https://www.postgresql.org/docs/current/app-initdb.html).

Notebook chạy theo thứ tự:

1. Preflight, download/build và kiểm version.
2. Build/test/contract Go và cài matplotlib phiên bản khóa.
3. Native integration/fault/smoke, tự tạo và dọn dependency.
4. Suite SQL và Redis+SQL, hai replica/mode.
5. Hiển thị biểu đồ, lưu metadata, log, JSON/CSV và PNG/SVG.

Không đưa RabbitMQ hoặc nghiệp vụ mock vào đường đo acceptance. Toàn bộ service chạy trên loopback, không cần public endpoint/ngrok. Bộ kiểm tra nhanh vẫn smoke đủ 5 service trước benchmark.

## Bằng chứng và xử lý lỗi

Bấm **Save & Run All**; chỉ đánh dấu kiểm chứng Kaggle hoàn tất nếu toàn bộ notebook kết thúc thành công trong phiên mới. Giữ thư mục `/kaggle/working/ride-share-results/run-*` và notebook version. Bootstrap và native integration/race/fault/smoke đã qua trong container Linux amd64 chạy root với Go/PostgreSQL/Redis đã khóa; điều này kiểm chứng source build và quyền nobody, chưa thay thế lần thực thi trực tiếp trên tài khoản Kaggle.

- Không tải được source: kiểm tra Internet và URL trong versions lock; checksum sai thì dừng, không tự bỏ kiểm tra.
- Build thiếu compiler hoặc quyền chạy process: preflight báo lỗi; không chuyển sang DB/mock rồi tiếp tục benchmark.
- Port bị dùng: runner dừng và giữ log; không kill tiến trình không thuộc runner.
- Cell bị exception: `finally` dừng các process runner sở hữu. Nếu notebook bị runtime kill hoặc dừng cưỡng bức, restart session trước khi chạy lại.
- Root runtime không chạy được PostgreSQL: kiểm tra `runuser`, account `nobody`, quyền traverse thư mục source/run và log initdb.
- Không có Internet: notebook chuẩn yêu cầu Internet; cần chuẩn bị riêng dataset toolchain/source/module cache nếu muốn hỗ trợ offline, chưa thuộc phiên bản này.

Checksum lấy từ [PostgreSQL release](https://ftp.postgresql.org/pub/source/v16.10/postgresql-16.10.tar.bz2.sha256), [Redis release hashes](https://github.com/redis/redis-hashes), [Go downloads](https://go.dev/dl/?mode=json&include=all).
