#!/bin/bash

# 配置文件路径
NGINX_CONF="/www/server/nginx/conf/nginx.conf"
BACKUP_DIR="/www/server/nginx/backups"
BACKUP_FILE="$BACKUP_DIR/nginx.conf.bak_$(date +%F_%T)"

# SSL 证书和私钥文件路径
SSL_DIR="/www/server/panel/vhost/cert"

# 检查备份目录是否存在，不存在则创建
if [ ! -d "$BACKUP_DIR" ]; then
    mkdir -p "$BACKUP_DIR" || { echo "创建备份目录失败！"; exit 1; }
    echo "备份目录 $BACKUP_DIR 创建成功。"
fi

# 备份原 nginx.conf 文件
if [ -f "$NGINX_CONF" ]; then
    cp "$NGINX_CONF" "$BACKUP_FILE" || { echo "备份失败！"; exit 1; }
    echo "原有的 nginx.conf 文件已备份为 $BACKUP_FILE"
else
    echo "未找到原有的 nginx.conf 文件，无法备份。" >&2
    exit 1
fi

echo "请将证书文件放入 $SSL_DIR 格式：127.0.0.2.crt 127.0.0.2.key"

# 获取输入的 TCP 负载均衡的服务器 IP 和端口
echo "请输入 TCP 负载均衡的服务器 IP 和端口（多个服务器用空格分隔，例如 403.45.64.135:1234 503.45.64.135:1234）："
read -a backend_ips

# 获取普通 TCP 负载均衡的监听端口
echo "请输入普通 TCP 负载均衡的监听端口（例如 25125）："
read tcp_port

# 获取带证书的 TLS 加密的 TCP 负载均衡的监听端口
echo "请输入带证书的 TLS 加密的 TCP 负载均衡的监听端口（例如 25126）："
read ssl_port

# 获取本机 IP 地址（可以多个）
echo "请输入本机的 IP 地址（多个用空格分隔，例如 127.0.0.1:12444 127.0.0.2）："
read -a local_ips

# 开始生成 nginx.conf 配置文件
{
    echo "user www www;"
    echo "worker_processes auto;"
    echo "error_log  /www/wwwlogs/nginx_error.log  crit;"
    echo "pid        /www/server/nginx/logs/nginx.pid;"
    echo "worker_rlimit_nofile 51200;"
    echo ""
    echo "stream {"
    echo "    log_format tcp_format '\$time_local|\$remote_addr|\$protocol|\$status|\$bytes_sent|\$bytes_received|\$session_time|\$upstream_addr|\$upstream_bytes_sent|\$upstream_bytes_received|\$upstream_connect_time';"
    echo ""
    echo "    # TCP 服务器池"
    echo "    upstream backend_servers {"

    # 将用户输入的后端服务器 IP 和端口添加到配置中
    for ip in "${backend_ips[@]}"; do
        # 验证服务器 IP 和端口格式
        if [[ ! "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:[0-9]+$ ]]; then
            echo "        # 错误：无效的服务器格式: $ip" >&2
            continue
        fi
        echo "        server $ip;"
    done

    echo "    }"
    echo ""
    echo "    # 普通 TCP 负载均衡配置"
    echo "    server {"
    echo "        listen $tcp_port;  # 普通 TCP 负载均衡监听端口"
    echo "        proxy_pass backend_servers;  # 转发到后端服务器"
    echo ""
    echo "        # TCP 代理超时设置"
    echo "        proxy_timeout 1m;  # 设置代理的超时时间，避免长时间未响应的连接"
    echo "        proxy_connect_timeout 2s;  # 设置连接后端的超时时间"
    echo "        tcp_nodelay on;  # 启用 TCP_NODELAY，避免延迟"
    echo ""
    echo "        # 启用代理缓冲区，避免过多的网络延迟"
    echo "        proxy_buffer_size 16k;  # 设置初始缓冲区大小"
    echo "    }"
    echo ""

    # 遍历本机 IP，查找证书
    for ip in "${local_ips[@]}"; do
        # 使用正则表达式提取 IP 和端口
        if [[ "$ip" =~ ^([^:]+):([0-9]+)$ ]]; then
            ip_address="${BASH_REMATCH[1]}"
            ip_port="${BASH_REMATCH[2]}"
        elif [[ "$ip" =~ ^([^:]+)$ ]]; then
            ip_address="${BASH_REMATCH[1]}"
            ip_port="$ssl_port"
        else
            echo "    # 无效的本机 IP 格式: $ip" >&2
            continue
        fi

        cert_file="${SSL_DIR}/${ip_address}.crt"
        key_file="${SSL_DIR}/${ip_address}.key"

        if [[ -f "$cert_file" && -f "$key_file" ]]; then
            echo "    # 带证书的 TLS 加密的 TCP 负载均衡 for $ip_address:$ip_port"
            echo "    server {"
            echo "        listen $ip_address:$ip_port ssl fastopen=256;  # 启用 SSL/TLS 加密，指定端口"
            echo "        proxy_pass backend_servers;  # 转发到后端服务器"
            echo ""
            echo "        # SSL 配置部分"
            echo "        ssl_certificate $cert_file;  # 证书路径"
            echo "        ssl_certificate_key $key_file;  # 私钥路径"
            echo "        ssl_session_cache shared:SSL:10m;"
            echo "        ssl_session_timeout 1d;"
            echo "        ssl_protocols TLSv1.2 TLSv1.3;  # 确保启用 TLS 1.2 和 TLS 1.3"
            echo "        ssl_ciphers 'TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-RSA-CHACHA20-POLY1305';"
            echo ""
            echo "        ssl_prefer_server_ciphers on;"
            echo "        ssl_verify_client off;  # 关闭客户端证书验证，如果需要开启，请调整配置"
            echo ""
            echo "        # TCP 代理超时设置"
            echo "        proxy_timeout 1m;  # 设置代理的超时时间，避免长时间未响应的连接"
            echo "        proxy_connect_timeout 2s;  # 设置连接后端的超时时间"
            echo "        tcp_nodelay on;  # 启用 TCP_NODELAY，避免延迟"
            echo ""
            echo "        # 启用代理缓冲区，避免过多的网络延迟"
            echo "        proxy_buffer_size 16k;  # 设置初始缓冲区大小"
            echo "    }"
            echo ""
        else
            echo "    # 未找到与服务器 IP $ip_address 对应的证书，跳过该 IP 的 TLS 配置。"
        fi
    done

    # 移除或注释掉日志相关的配置（根据“去除log”的要求）
    echo "    access_log /www/wwwlogs/tcp-access.log tcp_format;"
    echo "    error_log /www/wwwlogs/tcp-error.log;"
    echo "    # access_log /www/wwwlogs/tcp-access.log tcp_format;"
    echo "    # error_log /www/wwwlogs/tcp-error.log;"
    echo "    include /www/server/panel/vhost/nginx/tcp/*.conf;"
    echo "}"
    echo ""
    echo "events {"
    echo "    use epoll;"
    echo "    worker_connections 51200;"
    echo "    multi_accept on;"
    echo "}"
    echo ""
    echo "http {"
    echo "    include       mime.types;"
    echo "    include proxy.conf;"
    echo "    lua_package_path \"/www/server/nginx/lib/lua/?.lua;;\";"
    echo ""
    echo "    default_type  application/octet-stream;"
    echo ""
    echo "    server_names_hash_bucket_size 512;"
    echo "    client_header_buffer_size 32k;"
    echo "    large_client_header_buffers 4 32k;"
    echo "    client_max_body_size 50m;"
    echo ""
    echo "    sendfile   on;"
    echo "    tcp_nopush on;"
    echo ""
    echo "    keepalive_timeout 60;"
    echo ""
    echo "    tcp_nodelay on;"
    echo ""
    echo "    fastcgi_connect_timeout 300;"
    echo "    fastcgi_send_timeout 300;"
    echo "    fastcgi_read_timeout 300;"
    echo "    fastcgi_buffer_size 64k;"
    echo "    fastcgi_buffers 4 64k;"
    echo "    fastcgi_busy_buffers_size 128k;"
    echo "    fastcgi_temp_file_write_size 256k;"
    echo "    fastcgi_intercept_errors on;"
    echo ""
    echo "    gzip on;"
    echo "    gzip_min_length  1k;"
    echo "    gzip_buffers     4 16k;"
    echo "    gzip_http_version 1.1;"
    echo "    gzip_comp_level 2;"
    echo "    gzip_types     text/plain application/javascript application/x-javascript text/javascript text/css application/xml application/json image/jpeg image/gif image/png font/ttf font/otf image/svg+xml application/xml+rss text/x-js;"
    echo "    gzip_vary on;"
    echo "    gzip_proxied   expired no-cache no-store private auth;"
    echo "    gzip_disable   \"MSIE [1-6]\\.\";"
    echo ""
    echo "    limit_conn_zone \$binary_remote_addr zone=perip:10m;"
    echo "    limit_conn_zone \$server_name zone=perserver:10m;"
    echo ""
    echo "    server_tokens off;"
    echo "    access_log /www/wwwlogs/access.log;"
    echo "    # access_log /www/wwwlogs/access.log;"
    echo ""
    echo "    server {"
    echo "        listen 888;"
    echo "        server_name phpmyadmin;"
    echo "        index index.html index.htm index.php;"
    echo "        root  /www/server/phpmyadmin;"
    echo ""
    echo "        include enable-php.conf;"
    echo ""
    echo "        location ~ .*\.(gif|jpg|jpeg|png|bmp|swf)\$ {"
    echo "            expires      30d;"
    echo "        }"
    echo ""
    echo "        location ~ .*\.(js|css)?\$ {"
    echo "            expires      12h;"
    echo "        }"
    echo ""
    echo "        location ~ /\. {"
    echo "            deny all;"
    echo "        }"
    echo ""
    echo "        access_log  /www/wwwlogs/access.log;"
    echo "        # access_log  /www/wwwlogs/access.log;"
    echo "    }"
    echo ""
    echo "    include /www/server/panel/vhost/nginx/*.conf;"
    echo "}"
} > "$NGINX_CONF"

echo "新的 nginx.conf 文件已经生成并输出到命令行。"

# 检查 Nginx 是否已安装
if ! command -v nginx &> /dev/null; then
    echo "错误：Nginx 未安装或未在 PATH 中。请安装 Nginx 或检查 PATH 设置。" >&2
    exit 1
fi

# 测试 Nginx 配置
echo "正在测试新的 Nginx 配置文件..."
if nginx -t; then
    echo "配置文件测试成功。正在重新加载 Nginx..."
    if nginx -s reload; then
        echo "Nginx 已成功重新加载。"
    else
        echo "错误：重新加载 Nginx 失败。" >&2
        exit 1
    fi
else
    echo "错误：Nginx 配置文件测试失败。请检查配置文件。" >&2
    exit 1
fi

