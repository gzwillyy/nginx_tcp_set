package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// 获取用户输入
func getUserInput(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

// 生成 Nginx 配置
func generateNginxConfig(tcpServers []string, tcpPort int, tlsPort int) string {
	// 拼接 TCP 服务器池部分
	var upstreamServers string
	for _, server := range tcpServers {
		upstreamServers += fmt.Sprintf("        server %s;\n", server)
	}

	// Nginx 配置模板
	configTemplate := fmt.Sprintf(`
user  www www;
worker_processes auto;
error_log  /www/wwwlogs/nginx_error.log  crit;
pid        /www/server/nginx/logs/nginx.pid;
worker_rlimit_nofile 51200;

stream {
    log_format tcp_format '$time_local|$remote_addr|$protocol|$status|$bytes_sent|$bytes_received|$session_time|$upstream_addr|$upstream_bytes_sent|$upstream_bytes_received|$upstream_connect_time';

    # TCP 服务器池
    upstream backend_servers {
%s
    }


    # 不加密的 TCP 负载均衡
    server {
        listen %d;  # 普通 TCP 负载均衡监听端口
        proxy_pass backend_servers;  # 转发到后端服务器
        # TCP 代理超时设置
        proxy_timeout 1m;  # 设置代理的超时时间，避免长时间未响应的连接
        proxy_connect_timeout 2s;  # 设置连接后端的超时时间
        tcp_nodelay on;  # 启用 TCP_NODELAY，避免延迟
        # 启用代理缓冲区，避免过多的网络延迟
        proxy_buffer_size 16k;  # 设置初始缓冲区大小
    }

    # 带证书的 TLS 加密的 TCP 负载均衡
    server {
        listen %d ssl fastopen=256;  # 启用 SSL/TLS 加密
        proxy_pass backend_servers;  # 转发到后端服务器

        # 允许读取 SNI 信息
        ssl_preread on;

        # 动态加载证书
		ssl_certificate_by_lua_file /www/server/nginx/lib/lua/dynamic_tls.lua;

        # SSL 配置部分
        ssl_session_cache shared:SSL:10m;
        ssl_session_timeout 1d;
        ssl_protocols TLSv1.2 TLSv1.3;  # 确保启用 TLS 1.2 和 TLS 1.3
        ssl_ciphers 'TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-RSA-CHACHA20-POLY1305';  # 添加 CHACHA20 套件

        ssl_prefer_server_ciphers on;
        ssl_verify_client off;  # 关闭客户端证书验证，如果需要开启，请调整配置

        # TCP 代理超时设置
        proxy_timeout 1m;  # 设置代理的超时时间，避免长时间未响应的连接
        proxy_connect_timeout 2s;  # 设置连接后端的超时时间
        tcp_nodelay on;  # 启用 TCP_NODELAY，避免延迟

        # 启用代理缓冲区，避免过多的网络延迟
        proxy_buffer_size 16k;  # 设置初始缓冲区大小
    }

    access_log /www/wwwlogs/tcp-access.log tcp_format;
    error_log /www/wwwlogs/tcp-error.log;
    include /www/server/panel/vhost/nginx/tcp/*.conf;
}

events {
    use epoll;
    worker_connections 51200;
    multi_accept on;
}

http {
	# 动态证书缓存
    lua_shared_dict cert_cache 20m;  # 存储证书和私钥缓存

    include       mime.types;
    #include luawaf.conf;

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
    access_log /www/wwwlogs/access.log;  # 设置日志输出

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

        location ~ /\.
        {
            deny all;
        }

        access_log  /www/wwwlogs/access.log;
    }

    include /www/server/panel/vhost/nginx/*.conf;
}
`, upstreamServers, tcpPort, tlsPort)

	return configTemplate
}

// 确保 dynamic_tls.lua 文件存在并写入代码
func ensureDynamicTLSFile() error {
	filePath := "/www/server/nginx/lib/lua/dynamic_tls.lua"
	// filePath := "./dynamic_tls.lua"

	// 检查文件是否存在
	if _, err := os.Stat(filePath); err == nil {
		// 文件存在，先删除文件
		_ = os.Remove(filePath)
	}

	// 文件不存在，创建并写入内容
	luaCode := `
	local _M = {}
	
	-- 动态加载证书和私钥
	function _M.load_cert()
		local server_ip = ngx.var.server_addr
	
		-- 证书和密钥路径
		local base_path = "/www/server/panel/vhost/cert/"
		local cert_path = base_path .. server_ip .. ".crt"
		local key_path = base_path .. server_ip .. ".key"
	
		-- 缓存键
		local cert_cache_key = "cert:" .. server_ip
		local key_cache_key = "key:" .. server_ip
	
		-- 从共享缓存获取证书和密钥
		local cert = ngx.shared.cert_cache:get(cert_cache_key)
		local key = ngx.shared.cert_cache:get(key_cache_key)
	
		-- 如果缓存中没有证书或密钥，从文件加载
		if not cert or not key then
			-- 文件读取函数
			local function load_file(path)
				local file, err = io.open(path, "r")
				if not file then
					ngx.log(ngx.ERR, "Failed to open file: ", path, " (Error: ", err, ")")
					return nil
				end
				local content = file:read("*a")
				file:close()
				return content
			end
	
			-- 从文件加载证书和密钥
			cert = load_file(cert_path)
			key = load_file(key_path)
	
			if cert and key then
				-- 缓存证书和密钥，设置有效期为 1 小时
				ngx.shared.cert_cache:set(cert_cache_key, cert, 3600)
				ngx.shared.cert_cache:set(key_cache_key, key, 3600)
			else
				ngx.log(ngx.ERR, "Certificate or key file not found for IP: ", server_ip)
				return ngx.exit(ngx.ERROR)
			end
		end
	
		-- 加载证书和密钥到 SSL
		local cert_obj, cert_err = ngx.ssl.parse_pem_cert(cert)
		local key_obj, key_err = ngx.ssl.parse_pem_priv_key(key)
	
		if not cert_obj then
			ngx.log(ngx.ERR, "Failed to parse certificate: ", cert_err)
			return ngx.exit(ngx.ERROR)
		end
	
		if not key_obj then
			ngx.log(ngx.ERR, "Failed to parse private key: ", key_err)
			return ngx.exit(ngx.ERROR)
		end
	
		ngx.ssl.clear_certs()
		ngx.ssl.set_cert(cert_obj)
		ngx.ssl.set_priv_key(key_obj)
	end
	
	return _M
	
	`
	// 创建并写入文件
	err := os.WriteFile(filePath, []byte(luaCode), 0644)
	if err != nil {
		return fmt.Errorf("无法创建或写入文件 %s: %v", filePath, err)
	}
	fmt.Println("文件 /www/server/nginx/lib/lua/dynamic_tls.lua 创建并写入成功")
	return nil
}

// 保存配置文件并替换
func saveAndReplaceConfig(configContent string, configPath string, backupDir string) error {
	// 生成备份文件路径和文件名
	backupFile := fmt.Sprintf("%s/nginx.conf.bak_%s", backupDir, time.Now().Format("2006-01-02_15-04-05"))

	// 备份现有配置
	err := os.Rename(configPath, backupFile)
	if err != nil {
		return fmt.Errorf("备份配置文件失败: %v", err)
	}
	fmt.Printf("现有配置文件已备份为 %s\n", backupFile)

	// 保存新配置
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}
	fmt.Println("配置文件已更新")
	return nil
}

func main() {
	// 在程序开始时检查并创建 dynamic_tls.lua 文件
	err := ensureDynamicTLSFile()
	if err != nil {
		fmt.Println("操作失败:", err)
		return
	}

	// 获取用户输入
	tcpServersInput := getUserInput("请输入 TCP 负载均衡的服务器 IP 和端口（多个用空格分隔，例如 403.45.64.135:1234 503.45.64.135:1234）：")
	tcpServers := strings.Fields(tcpServersInput)

	tcpPortInput := getUserInput("请输入普通 TCP 负载均衡的监听端口（例如 25125）：")
	tcpPort, _ := strconv.Atoi(tcpPortInput) // 转换为整数

	tlsPortInput := getUserInput("请输入带证书的 TLS 加密的 TCP 负载均衡的监听端口（例如 25126）：")
	tlsPort, _ := strconv.Atoi(tlsPortInput) // 转换为整数

	// 生成 Nginx 配置
	configContent := generateNginxConfig(tcpServers, tcpPort, tlsPort)

	// 备份并替换 Nginx 配置
	configPath := "/www/server/nginx/conf/nginx.conf"
	backupDir := "/www/server/nginx/backups"
	// configPath := "./conf/nginx.conf"
	// backupDir := "./backups"
	err = saveAndReplaceConfig(configContent, configPath, backupDir)
	if err != nil {
		fmt.Println("操作失败:", err)
		return
	}

	// 重新加载 Nginx 配置
	fmt.Println("重新加载 Nginx 配置...")
	err = exec.Command("nginx", "-s", "reload").Run()
	if err != nil {
		fmt.Println("Nginx 重载失败:", err)
	} else {
		fmt.Println("Nginx 配置重新加载成功！")
	}
}
