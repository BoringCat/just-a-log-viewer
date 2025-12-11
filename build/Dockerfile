FROM centos:7

# 你需要自己准备 _repos/CentOS-Base.repo
# mirrorlist.centos.org 不可用
# /etc/yum.repos.d/CentOS-Vault.repo 没有 7.9
# 推荐同时准备 _repos/epel.repo
COPY _repos/ /etc/yum.repos.d/

RUN --mount=type=cache,target=/var/cache set -xe\
 && yum makecache fast\
 && yum install -y epel-release --disablerepo=epel\
 && yum install -y '@Development Tools' systemd-devel.x86_64 glibc-devel.x86_64 brotli-devel.x86_64 make 

ARG GOVERSION=1.24.2
ARG GOARCH=amd64
RUN set -xe\
 && curl -SL https://go.dev/dl/go${GOVERSION}.linux-${GOARCH}.tar.gz > /tmp/go.tar.gz\
 && tar -C /usr/local/ -xf /tmp/go.tar.gz\
 && rm /tmp/go.tar.gz

ENV PATH=/usr/local/go/bin:${PATH}
