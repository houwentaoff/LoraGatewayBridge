# 使用说明

## 证书 
### 说明
支持SSL/TLS单向认证和双向认证(可选)，支持用户名密码模式（可选）支持mqtt,https,coap.

#### 单向认证
需要一个证书相关文件；包含服务器的CA证书（`ca.crt`）。此时网关只需要服务端的CA证书。

#### 双向认证
需要3个文件；包含服务器的CA证书、网关的CA证书和网关的私钥(`ca.crt, gwclient.crt, gwclient.key`)。

#####工作原理
1. 先确保服务器是非伪造的指定服务器，然后服务器会确保连接的客户端是非伪造指定的客户端。
2. 认证通过后服务器和网关之间的通信是通过动态生成的对称秘钥进行加密，对称秘钥通过非对称的秘钥进行加解密。

### 需要的材料
1. 服务器证书（`ca.crt`），客户端证书（`gwclient.crt`）和客户端私钥(`gwclient.key`)(`均从云平台下载`)
2. 用户账号和密码

### 如何 配置网关
1. 配置`global_conf.json`
   将其中的`server_address`字段改为`"server_address": "127.0.0.1"`映射到本地lora-gateway-bridge
   将端口`serv_port_up serv_port_down`字段分别改为`"serv_port_down":1700`, `"serv_port_up":1700`映射到本地lora-gateway-bridge的端口		
2. 将从服务器下载的ca证书放入目录`/mnt/data/ca`（下面命令会使用到）
3. 执行命令运行`lora-gateway-bridge` 其中`ssl://10.2.15.136:1883`为MQTT服务器对应的IP和端口,ssl表明使用SSL/TLS协议。
   `/mnt/data/lora-gateway-bridge    --mqtt-server  ssl://10.2.15.136:1883 --mqtt-username Jztest --mqtt-password Jz_2017 --mqtt-ca-cert /mnt/data/ca/ca.crt --mqtt-cli-ca-cert /mnt/data/ca/gwclient.crt  --mqtt-cli-key /mnt/data/ca/gwclient.key`

### 其它
1. 单向认证`/mnt/data/lora-gateway-bridge    --mqtt-server  ssl://10.2.15.136:1883 --mqtt-username Jztest --mqtt-password Jz_2017 --ca-cert /mnt/data/ca/ca.crt`
2. 双向认证`/mnt/data/lora-gateway-bridge    --mqtt-server  ssl://10.2.15.136:1883 --mqtt-username Jztest --mqtt-password Jz_2017 --ca-cert /mnt/data/ca/ca.crt --cli-ca-cert /mnt/data/ca/gwclient.crt  --cli-key /mnt/data/ca/gwclient.key`
3. 不使用认证只使用账户名密码`/mnt/data/lora-gateway-bridge    --mqtt-server  tcp://10.2.15.136:1883 --mqtt-username Jztest --mqtt-password Jz_2017`
4. 既不使用认证也不使用账户名密码`/mnt/data/lora-gateway-bridge    --mqtt-server  tcp://10.2.15.136:1883`

5. 使用coap `/mnt/data/lora-gateway-bridge --coap-server udp://127.0.0.1:5683`
6. 使用https 不认证 `/mnt/data/lora-gateway-bridge --https-server https://127.0.0.1:443`
7. 使用https  单向认证`/mnt/data/lora-gateway-bridge --https-server https://127.0.0.1:443 --ca-cert /mnt/data/ca/ca.crt`
8. 使用https 双向认证`/mnt/data/lora-gateway-bridge --https-server https://127.0.0.1:443 --ca-cert /mnt/data/ca/ca.crt --cli-ca-cert /mnt/data/ca/gwclient.crt  --cli-key /mnt/data/ca/gwclient.key`

## 备注
1. 使用`./lora-gateway-bridge -h`查看命令帮助如下
```bash
# ./lora-gateway-bridge  -h
NAME:
   lora-gateway-bridge - abstracts the packet_forwarder protocol into JSON over MQTT , COAP or HTTPS

USAGE:
   lora-gateway-bridge [global options] command [command options] [arguments...]

VERSION:
   1.01

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --udp-bind value       ip:port to bind the UDP listener to (default: "0.0.0.0:1700") [$UDP_BIND]
   --https-server value   https server (e.g. scheme://host:port where scheme is https (https://127.0.0.1:443)) [$HTTPS_SERVER]
   --coap-server value    coap server (e.g. scheme://host:port where scheme is udp (udp://127.0.0.1:5683)) [$COAP_SERVER]
   --mqtt-server value    mqtt server (e.g. scheme://host:port where scheme is tcp, ssl or ws(tcp://127.0.0.1:1883)) [$MQTT_SERVER]
   --mqtt-username value  mqtt server username (optional) [$MQTT_USERNAME]
   --mqtt-password value  mqtt server password (optional) [$MQTT_PASSWORD]
   --ca-cert value        CA certificate file (optional) [$CA_CERT]
   --cli-ca-cert value    cli-CA certificate file (optional) [$CLI_CA_CERT]
   --cli-key value        cli-KEY pri key file (optional) [$CLI_KEY]
   --skip-crc-check       skip the CRC status-check of received packets [$SKIP_CRC_CHECK]
   --log-level value      debug=5, info=4, warning=3, error=2, fatal=1, panic=0 (default: 4) [$LOG_LEVEL]
   --help, -h             show help
   --version, -v          print the version

COPYRIGHT:
   Joy Hou

```
2. 其中是否需要账户名密码CA证书均由用户在云平台设置时候选定和修改，详情参考云平台lora使用说明。
