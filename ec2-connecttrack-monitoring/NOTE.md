# EC2 connect track monitoring

## 说明
每个EC2实例支持不同的[在线连接数](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/security-group-connection-tracking.html)。当系统繁忙超过链接数时新tcp连接将无法建立。

AWS EC2通过通过ethtool工具从ENA模块上获取当前可用的connect track的信息，以及超出部分的信息。下面将详细说明如何获取及监控在线连接数信息：

**需要确保ENA模块在2.8.1版本及以上**

```azure
sudo ethtool -i ens5
driver: ena
version: 2.12.3g
firmware-version:
    expansion-rom-version:
    bus-info: 0000:00:05.0
supports-statistics: yes
supports-test: no
supports-eeprom-access: no
supports-register-dump: no
supports-priv-flags: yes
```

查询指定接口的可用connect track数量：

```azure
sudo ethtool -S ens5 | grep conntrack_allowance
     conntrack_allowance_exceeded: 0
     conntrack_allowance_available: 307829
```

获取当前系统有多少个连接数。

```azure
ss -A tcp,udp state all | awk 'NR > 1 && $6 !~ /0.0.0.0:\*/ && $6 !~ /\[::\]:\*/ && $6 !~ /127.0.0.1:/ && $6 !~ /\[::ffff:127.0.0.1\]/ && $6 != "*:*" {print $6}' | wc -l


```

评估当前EC2实例支持多少connect track (ec2_connection.sh)：

**前提条件：**
1. 当前EC2没有符合[Untracked connections](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/security-group-connection-tracking.html#untracked-connections)的安全组规则：

Not all flows of traffic are tracked. If a security group rule permits TCP or UDP flows for all traffic (0.0.0.0/0 or ::/0) and there is a corresponding rule in the other direction that permits all response traffic (0.0.0.0/0 or ::/0) for any port (0-65535), then that flow of traffic is not tracked, unless it is part of an [automatically tracked connection](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/security-group-connection-tracking.html#automatic-tracking).

2. 当前EC2的ENA模块支持获取可用connectrack

```azure
#!/bin/bash

# Get the list of all network interfaces
interfaces=$(ip -o link show | awk -F': ' '{print $2}')

# Initialize total conntrack allowance available
total_conntrack_available=0

# Loop through each interface and sum conntrack_allowance_available
for iface in $interfaces; do
    conntrack_available=$(sudo ethtool -S $iface 2>/dev/null | grep conntrack_allowance_available | awk '{print $2}')
    if [ ! -z "$conntrack_available" ]; then
        total_conntrack_available=$((total_conntrack_available + conntrack_available))
    fi
done

# Get the current number of connections
conntrack_current=$(ss -A tcp,udp state all | awk 'NR > 1 && $6 !~ /0.0.0.0:\*/ && $6 !~ /\[::\]:\*/ && $6 !~ /127.0.0.1:/ && $6 !~ /\[::ffff:127.0.0.1\]/ && $6 != "*:*" {print $6}' | wc -l)

# Calculate the total number of connections supported
total_connections=$((total_conntrack_available + conntrack_current))

# Output the result
echo "Total number of connections supported: $total_connections"
```

## 监控方式 （node_exporter)

[node_exporter](https://github.com/prometheus/node_exporter/blob/master/collector/ethtool_linux.go)支持采集ethtool的conntrack_allowance_exceeded和conntrack_allowance_available指标；

开启采集connetrack:

```azure
 --collector.ethtool
```

要统计现有connetrack占比：

PromQL:
```azure
( 1 - node_ethtool_conntrack_allowance_available{device="ens5"} / <supported conntrack_count> ) * 100
```



## 监控方式 （Cloudwatch Agent）
https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-network-performance.html

## Appendix

### 官方说明
Blog:

https://aws.amazon.com/blogs/networking-and-content-delivery/monitoring-ec2-connection-tracking-utilization-using-a-new-network-performance-metric/

### ENA模块相关：
update ENA module:

https://github.com/amzn/amzn-drivers/tree/master/kernel/linux/ena

### node_exporter演示环境构建


#### docker-compose.yml

```azure
version: '3.8'

networks:
  monitoring:
    driver: bridge

volumes:
  prometheus_data: {}
  grafana_data: {}

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--web.enable-lifecycle'
    expose:
      - 9090
    networks:
      - monitoring

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    volumes:
      - grafana_data:/var/lib/grafana
    ports:
      - 3000:3000
    networks:
      - monitoring
```

#### prometheus.yml

```azure
global:
  scrape_interval: 1m

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'node'
    static_configs:
      - targets: ['<HOST IP>:9100']
```

```azure
docker-compose up -d
```

#### Node_exporter

1. Download

https://prometheus.io/download/

2. Install

```azure
tar xvf node_exporter-*.tar.gz
```

3. Enable and Start Node Exporter as service

Create a systemd service file: /etc/systemd/system/node_exporter.service
```azure
[Unit]
Description=Node Exporter
Wants=network-online.target
After=network-online.target

[Service]
User=nobody
ExecStart=/path/to/node_exporter --collector.disable-defaults --collector.ethtool
Restart=always

[Install]
WantedBy=default.target

```

Enable and start the service
```azure
sudo systemctl daemon-reload
sudo systemctl start node_exporter
sudo systemctl enable node_exporter
```

Verify:

```azure
curl http://<HOST IP>:9100/metrics
```

