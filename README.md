build_tcp_conf.sh：对宝塔编译安装的nginx，进行 tcp 负载均衡配置

optimize_system.sh：优化系统对 tcp 支持


```sh
# 通用安装bt
if [ -f /usr/bin/curl ];then curl -sSO http://io.bt.sb/install/install_panel.sh;else wget -O install_panel.sh http://io.bt.sb/install/install_panel.sh;fi;bash install_panel.sh


# debian ubuntu 安装bt
wget -O install.sh http://io.bt.sb/install/install-ubuntu_6.0.sh && bash install.sh

# centos 安装bt
yum install -y wget && wget -O install.sh http://io.bt.sb/install/install_6.0.sh && sh install.sh

# 升级为企业
curl https://io.bt.sb/install/update_panel.sh|bash
```


```
curl -L -o build_tcp_conf  https://github.com/gzwillyy/nginx_tcp_set/raw/master/build_tcp_conf

curl -sSL https://github.com/gzwillyy/nginx_tcp_set/raw/master/optimize_system.sh | bash
```