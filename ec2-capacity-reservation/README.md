# 自动从EC2资源池中预留资源脚本

## 工作原理

在脚本中需要定义如下参数：

    region = 'ap-east-1'
    instance_type = "m6i.8xlarge"
    availability_zone_id = "ape1-az1"
    max_reservations = 10
    counter_file_base = "/home/admin/tmp/reservation_count"  
    log_file_base = "/home/admin/tmp/capacity_reservation_log" 
    reservation_tag = "my-capacity-reservation"

脚本会在指定region中预留max_reservations 参数中定义的机器数量。如果当前资源不足，重复运行脚本时，脚本会根据剩余未开机器的数量开机。 

通过定义reservation_tag内容区分不同的预留机型。

## 安装部署

### 设置环境
```
python3 -m venv reserve_ec2

source reserve_ec2/bin/activate

pip3 install boto3
```

### 测试运行

```
python3 reserve_ec2.py
```

### 配置crontab

```
*/5 * * * * <my_project_venv>/bin/python3 <scriptpath>/reserve_ec2.py >> <logpath>/capacity_reservation_log_my-capacity-reservation.txt 2>&1
```

