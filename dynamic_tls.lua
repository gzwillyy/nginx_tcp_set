
-- dynamic_tls.lua
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

