#!/bin/bash

# 显示中文提示信息
echo "==========================================="
echo "正在进行操作系统调优，优化 TCP 连接性能。"
echo "==========================================="

# Step 1: 启用 TCP Fast Open
echo "步骤 1：启用 TCP Fast Open..."
echo 3 > /proc/sys/net/ipv4/tcp_fastopen
echo "net.ipv4.tcp_fastopen=3" >> /etc/sysctl.conf
sysctl -p
echo "TCP Fast Open 已启用，配置已写入 /etc/sysctl.conf"

# Step 2: 优化 TCP 缓冲区设置
echo "步骤 2：优化 TCP 缓冲区设置..."
echo "net.core.rmem_max=16777216" >> /etc/sysctl.conf
echo "net.core.wmem_max=16777216" >> /etc/sysctl.conf
echo "net.ipv4.tcp_rmem=4096 87380 16777216" >> /etc/sysctl.conf
echo "net.ipv4.tcp_wmem=4096 87380 16777216" >> /etc/sysctl.conf
sysctl -p
echo "TCP 缓冲区已优化，配置已写入 /etc/sysctl.conf"

# Step 3: 增加文件描述符限制
echo "步骤 3：增加文件描述符限制..."
echo "* soft nofile 65535" >> /etc/security/limits.conf
echo "* hard nofile 65535" >> /etc/security/limits.conf

# 适配不同系统处理 pam_limits
# Ubuntu/Debian 系统默认启用 pam_limits，CentOS 系统则需要明确添加
if [ -f /etc/pam.d/common-session ]; then
    echo "session required pam_limits.so" >> /etc/pam.d/common-session
    echo "pam_limits 配置已添加到 /etc/pam.d/common-session"
else
    echo "警告：/etc/pam.d/common-session 文件未找到，可能是 CentOS 系统，手动检查此文件"
fi

echo "文件描述符限制已增加，配置已写入 /etc/security/limits.conf"

# Step 4: 提高系统最大连接数和 TCP 连接队列大小
echo "步骤 4：提高系统最大连接数和 TCP 连接队列大小..."
echo "fs.file-max = 1000000" >> /etc/sysctl.conf
echo "net.core.somaxconn = 65535" >> /etc/sysctl.conf
sysctl -p
echo "系统最大连接数和 TCP 连接队列大小已提高，配置已写入 /etc/sysctl.conf"

# 提示重启系统生效
echo "==========================================="
echo "系统调优完成，部分设置已生效。"
echo "请重启系统以使所有更改生效。"
echo "==========================================="
