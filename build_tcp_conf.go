package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 检查并创建目录
func createDirIfNotExists(dir string) error {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}
		fmt.Printf("备份目录 %s 创建成功。\n", dir)
	} else if err != nil {
		return fmt.Errorf("检查目录失败: %v", err)
	}
	return nil
}

// 获取用户输入
func getUserInput(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

// 获取多个用户输入，按空格分隔
func getMultipleUserInput(prompt string) []string {
	input := getUserInput(prompt)
	return strings.Fields(input)
}

// 备份文件
func backupFile(src, dest string) error {
	inputFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("无法打开源文件 %s: %v", src, err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("无法创建备份文件 %s: %v", dest, err)
	}
	defer outputFile.Close()

	_, err = outputFile.ReadFrom(inputFile)
	if err != nil {
		return fmt.Errorf("备份文件失败: %v", err)
	}

	fmt.Printf("原有的 nginx.conf 文件已备份为 %s\n", dest)
	return nil
}

// 检查 Nginx 是否安装
func isNginxInstalled() bool {
	_, err := exec.LookPath("nginx")
	return err == nil
}

// 测试 Nginx 配置文件
func testNginxConfig() error {
	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Nginx 配置文件测试失败: %v\n%s", err, output)
	}
	return nil
}

// 重新加载 Nginx
func reloadNginx() error {
	cmd := exec.Command("nginx", "-s", "reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重新加载 Nginx 失败: %v\n%s", err, output)
	}
	fmt.Println("Nginx 已成功重新加载。")
	return nil
}

// 生成新的 nginx.conf 配置文件
func generateNginxConf(nginxConf, backupDir, sslDir string, backendIPs, localIPs []string, tcpPort, sslPort string) error {
	// 生成备份文件路径
	backupFileName := filepath.Join(backupDir, "nginx.conf.bak_"+time.Now().Format("2006-01-02_15:04:05"))
	if err := backupFile(nginxConf, backupFileName); err != nil {
		return err
	}

	// 创建新配置文件
	f, err := os.Create(nginxConf)
	if err != nil {
		return fmt.Errorf("无法创建 nginx.conf 文件: %v", err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)

	// 固定的 nginx.conf 配置（events、http、server）
	fmt.Fprintf(writer, `user  www www;
worker_processes auto;
error_log  /www/wwwlogs/nginx_error.log  crit;
pid        /www/server/nginx/logs/nginx.pid;
worker_rlimit_nofile 51200;

`)

	// 后端服务器配置
	fmt.Fprintf(writer, "stream {\n")
	fmt.Fprintf(writer, "    log_format tcp_format '$time_local|$remote_addr|$protocol|$status|$bytes_sent|$bytes_received|$session_time|$upstream_addr|$upstream_bytes_sent|$upstream_bytes_received|$upstream_connect_time';\n")
	fmt.Fprintf(writer, "    upstream backend_servers {\n")

	for _, ip := range backendIPs {
		if !strings.Contains(ip, ":") {
			fmt.Fprintf(writer, "        # 错误：无效的服务器格式: %s\n", ip)
			continue
		}
		fmt.Fprintf(writer, "        server %s;\n", ip)
	}

	fmt.Fprintf(writer, "    }\n\n")

	// 普通 TCP 负载均衡配置
	fmt.Fprintf(writer, "    server {\n")
	fmt.Fprintf(writer, "        listen %s;  # 普通 TCP 负载均衡监听端口\n", tcpPort)
	fmt.Fprintf(writer, "        proxy_pass backend_servers;  # 转发到后端服务器\n\n")
	fmt.Fprintf(writer, "        # TCP 代理超时设置\n")
	fmt.Fprintf(writer, "        proxy_timeout 1m;  # 设置代理的超时时间\n")
	fmt.Fprintf(writer, "        proxy_connect_timeout 2s;  # 设置连接后端的超时时间\n")
	fmt.Fprintf(writer, "        tcp_nodelay on;  # 启用 TCP_NODELAY，避免延迟\n\n")
	fmt.Fprintf(writer, "        # 启用代理缓冲区\n")
	fmt.Fprintf(writer, "        proxy_buffer_size 16k;  # 设置初始缓冲区大小\n")
	fmt.Fprintf(writer, "    }\n\n")

	// 遍历本机 IP，查找证书
	// 遍历本机 IP，查找证书并生成相应的 TLS 配置
	for _, ip := range localIPs {
		var ipAddress, ipPort string
		if strings.Contains(ip, ":") {
			parts := strings.Split(ip, ":")
			ipAddress, ipPort = parts[0], parts[1]
		} else {
			ipAddress, ipPort = ip, sslPort // 如果没有指定端口，使用用户输入的默认 sslPort
		}

		certFile := fmt.Sprintf("%s/%s.crt", sslDir, ipAddress)
		keyFile := fmt.Sprintf("%s/%s.key", sslDir, ipAddress)

		// 分开判断 cert 和 key 文件是否存在
		_, certErr := os.Stat(certFile)
		_, keyErr := os.Stat(keyFile)

		if certErr == nil && keyErr == nil {
			// 如果证书和私钥都存在，为该 IP 地址生成一个 TLS 加密的 TCP 负载均衡配置
			fmt.Fprintf(writer, "    # 带证书的 TLS 加密的 TCP 负载均衡 for %s:%s\n", ipAddress, ipPort)
			fmt.Fprintf(writer, "    server {\n")
			fmt.Fprintf(writer, "        listen %s:%s ssl fastopen=256;  # 启用 SSL/TLS 加密\n", ipAddress, ipPort)
			fmt.Fprintf(writer, "        proxy_pass backend_servers;  # 转发到后端服务器\n\n")
			fmt.Fprintf(writer, "        # SSL 配置部分\n")
			fmt.Fprintf(writer, "        ssl_certificate %s;  # SSL 证书\n", certFile)
			fmt.Fprintf(writer, "        ssl_certificate_key %s;  # SSL 私钥\n", keyFile)
			fmt.Fprintf(writer, "        ssl_session_cache shared:SSL:10m;\n")
			fmt.Fprintf(writer, "        ssl_session_timeout 1d;\n")
			fmt.Fprintf(writer, "        ssl_protocols TLSv1.2 TLSv1.3;  # 确保启用 TLS 1.2 和 TLS 1.3\n")
			fmt.Fprintf(writer, "        ssl_ciphers 'TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-RSA-CHACHA20-POLY1305';  # 添加 CHACHA20 套件\n")
			fmt.Fprintf(writer, "        ssl_prefer_server_ciphers on;\n")
			fmt.Fprintf(writer, "        ssl_verify_client off;  # 关闭客户端证书验证，如果需要开启，请调整配置\n")
			fmt.Fprintf(writer, "        \n")
			fmt.Fprintf(writer, "        # TCP 代理超时设置\n")
			fmt.Fprintf(writer, "        proxy_timeout 1m;  # 设置代理的超时时间，避免长时间未响应的连接\n")
			fmt.Fprintf(writer, "        proxy_connect_timeout 2s;  # 设置连接后端的超时时间\n")
			fmt.Fprintf(writer, "        tcp_nodelay on;  # 启用 TCP_NODELAY，避免延迟\n")
			fmt.Fprintf(writer, "        # 启用代理缓冲区，避免过多的网络延迟\n")
			fmt.Fprintf(writer, "        proxy_buffer_size 16k;  # 设置初始缓冲区大小\n")

			fmt.Fprintf(writer, "    }\n\n")
		} else {
			// 如果没有找到证书或私钥，可以选择打印警告
			fmt.Printf("警告：证书或私钥缺失：%s 或 %s\n", certFile, keyFile)
		}
	}

	fmt.Fprintf(writer, "    access_log /www/wwwlogs/tcp-access.log tcp_format;\n")
	fmt.Fprintf(writer, "    error_log /www/wwwlogs/tcp-error.log;\n")
	fmt.Fprintf(writer, "    include /www/server/panel/vhost/nginx/tcp/*.conf;\n")
	fmt.Fprintf(writer, "}\n") // End of stream section

	fmt.Fprintf(writer, `
events {
	use epoll;
	worker_connections 51200;
	multi_accept on;
}

http {
	include       mime.types;
	include proxy.conf;
	lua_package_path "/www/server/nginx/lib/lua/?.lua;;";

	default_type  application/octet-stream;
	server_names_hash_bucket_size 512;
	client_header_buffer_size 32k;
	large_client_header_buffers 4 32k;
	client_max_body_size 50m;

	sendfile   on;
	tcp_nopush on;

	keepalive_timeout 60;

	tcp_nodelay on;

	fastcgi_connect_timeout 300;
	fastcgi_send_timeout 300;
	fastcgi_read_timeout 300;
	fastcgi_buffer_size 64k;
	fastcgi_buffers 4 64k;
	fastcgi_busy_buffers_size 128k;
	fastcgi_temp_file_write_size 256k;
	fastcgi_intercept_errors on;

	gzip on;
	gzip_min_length  1k;
	gzip_buffers     4 16k;
	gzip_http_version 1.1;
	gzip_comp_level 2;
	gzip_types     text/plain application/javascript application/x-javascript text/javascript text/css application/xml application/json image/jpeg image/gif image/png font/ttf font/otf image/svg+xml application/xml+rss text/x-js;
	gzip_vary on;
	gzip_proxied   expired no-cache no-store private auth;
	gzip_disable   "MSIE [1-6]\.";

	limit_conn_zone $binary_remote_addr zone=perip:10m;
	limit_conn_zone $server_name zone=perserver:10m;

	server_tokens off;
	access_log /www/wwwlogs/access.log;

server {
		listen 888;
		server_name phpmyadmin;
		index index.html index.htm index.php;
		root  /www/server/phpmyadmin;

		#error_page   404   /404.html;
		include enable-php.conf;

		location ~ .*\.(gif|jpg|jpeg|png|bmp|swf)$ {
			expires      30d;
		}

		location ~ .*\.(js|css)?$ {
			expires      12h;
		}

		location ~ /\.. {
			deny all;
		}

		access_log  /www/wwwlogs/access.log;
	}

include /www/server/panel/vhost/nginx/*.conf;
}`)
	writer.Flush()

	fmt.Println("新的 nginx.conf 文件已经生成。")
	return nil
}

func main() {
	// 配置文件路径
	nginxConf := "/www/server/nginx/conf/nginx.conf"
	backupDir := "/www/server/nginx/backups"
	sslDir := "/www/server/panel/vhost/cert"

	// 创建备份目录
	if err := createDirIfNotExists(backupDir); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("请将证书文件放入 /www/server/panel/vhost/cert 格式：127.0.0.2.crt 127.0.0.2.key")
	// 获取后端服务器和端口
	backendIPs := getMultipleUserInput("请输入 TCP 负载均衡的服务器 IP 和端口（多个用空格分隔，例如 403.45.64.135:1234 503.45.64.135:1234）：")
	tcpPort := getUserInput("请输入普通 TCP 负载均衡的监听端口（例如 25125）：")
	sslPort := getUserInput("请输入带证书的 TLS 加密的 TCP 负载均衡的监听端口（例如 25126）：")
	localIPs := getMultipleUserInput("请输入本机的 IP 地址（多个用空格分隔，例如 127.0.0.1:12444 127.0.0.2）：")

	// 生成新的 nginx.conf 配置文件
	if err := generateNginxConf(nginxConf, backupDir, sslDir, backendIPs, localIPs, tcpPort, sslPort); err != nil {
		fmt.Println(err)
		return
	}

	// 检查 Nginx 是否安装
	if !isNginxInstalled() {
		fmt.Println("Nginx 未安装或未在 PATH 中。请安装 Nginx 或检查 PATH 设置。")
		return
	}

	// 测试配置并重新加载 Nginx
	if err := testNginxConfig(); err != nil {
		fmt.Println(err)
		return
	}

	if err := reloadNginx(); err != nil {
		fmt.Println(err)
		return
	}
}
