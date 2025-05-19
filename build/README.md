# 到处部署

为了支持systemd-journald日志查看，需要开启 cgo

为了避免 `GLIBC_X.XX not found` 的问题，用最低版本的gcc编译

同时满足条件的发行版......，🌿！CentOS 7

```bash
docker build -t just-a-log-viewer:builder build
# 一定得_开头，否则go会扫描它们
mkdir build/_cache build/_go
docker run --rm \
    -v $PWD:/app \
    -v $PWD/build/_cache:/.cache \
    -v $PWD/build/_go:/go \
    -w /app --user `id -u`:`id -g` \
    -it just-a-log-viewer:builder \
    bash -c 'make deps && make dist.linux.amd64 dist.linux.386'
```
