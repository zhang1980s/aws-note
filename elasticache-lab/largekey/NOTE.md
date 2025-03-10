# Large Key

## Install redis tool on debian
```
sudo apt-get install lsb-release curl gpg
curl -fsSL https://packages.redis.io/gpg | sudo gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg
sudo chmod 644 /usr/share/keyrings/redis-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/redis.list
sudo apt-get update
sudo apt install redis-tools
```

## Write 100000 random keys

```
redis-benchmark -h my-redis-cluster.vrpjxs.0001.apse1.cache.amazonaws.com -p 6379 -n 100000 -t set -r 100000 -q

```

## verify the large key

```
➜   redis-cli -h my-redis-cluster.vrpjxs.0001.apse1.cache.amazonaws.com -p 6379 --bigkeys

# Scanning the entire keyspace to find biggest keys as well as
# average sizes per key type.  You can use -i 0.1 to sleep 0.1 sec
# per 100 SCAN commands (not usually needed).

100.00% ||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||
Keys sampled: 63198

-------- summary -------

Total key length in bytes is 1011168 (avg len 16.00)

Biggest string found "key:000000057714" has 3 bytes

0 lists with 0 items (00.00% of keys, avg size 0.00)
0 hashs with 0 fields (00.00% of keys, avg size 0.00)
0 streams with 0 entries (00.00% of keys, avg size 0.00)
63198 strings with 189594 bytes (100.00% of keys, avg size 3.00)
0 sets with 0 members (00.00% of keys, avg size 0.00)
0 zsets with 0 members (00.00% of keys, avg size 0.00)
```

# Hot key

## using the LFU (Least Frequently Used) maxmemory 

https://redis.io/docs/latest/develop/reference/eviction/#eviction-policies

Change the value of maxmemory-policy to 'allkeys-lfu' in parameter group of redis cluster

## create a hot key scenario
```
redis-benchmark -h my-redis-cluster.vrpjxs.0001.apse1.cache.amazonaws.com -p 6379 \
-n 1000000 \
-r 100 \  # Limited key range to create hot keys
-P 50 \
--cluster \
-t set,get \
-q &  # Run in background


redis-benchmark -h elr10jemqn7ni0c5.vrpjxs.clustercfg.apse1.cache.amazonaws.com -p 6379 -n 1000000 -r 100 --cluster -t set,get -q &
```

```
# Target specific keys more frequently
redis-benchmark -h your-redis-endpoint -p 6379 \
  -n 2000000 \
  -r 10 \   # Very limited key range for hot keys
  -P 50 \
  --cluster \
  -t get \
  -q &


redis-benchmark -h elr10jemqn7ni0c5.vrpjxs.clustercfg.apse1.cache.amazonaws.com -p 6379 -n 2000000 -r 10 -P 50 --cluster -t get -q &
```


## verify to hot key

### Non-cluster mode
```
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache/largekey git:(master) ✗ 
➜   redis-benchmark -h my-redis-cluster.vrpjxs.0001.apse1.cache.amazonaws.com -p 6379 -n 2000000 -r 10 -P 50 -t get -q &
[1] 47508
WARNING: Could not fetch server CONFIG                                                                                                                                                                                                         
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache/largekey git:(master) ✗ 
GET: 1344086.00 requests per second, p50=1.663 msec                     


[1]  + 47508 done       redis-benchmark -h my-redis-cluster.vrpjxs.0001.apse1.cache.amazonaws.com -p 
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache/largekey git:(master) ✗ 
➜   redis-cli -h my-redis-cluster.vrpjxs.0001.apse1.cache.amazonaws.com -p 6379 --hotkeys                               

# Scanning the entire keyspace to find hot keys as well as
# average sizes per key type.  You can use -i 0.1 to sleep 0.1 sec
# per 100 SCAN commands (not usually needed).

100.00% ||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||
Keys sampled: 63234

-------- summary -------

hot key found with counter: 226 keyname: "key:000000000003"
hot key found with counter: 223 keyname: "key:000000000008"
hot key found with counter: 212 keyname: "key:000000000002"
hot key found with counter: 210 keyname: "key:000000000006"
hot key found with counter: 209 keyname: "key:000000000001"
hot key found with counter: 209 keyname: "key:000000000009"
hot key found with counter: 208 keyname: "key:000000000000"
hot key found with counter: 206 keyname: "key:000000000005"
hot key found with counter: 202 keyname: "key:000000000004"
hot key found with counter: 190 keyname: "key:000000000007"
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache/largekey git:(master) ✗ 
```

### Cluster mode

```
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache-lab git:(master) ✗ 
SET: 84976.21 requests per second, p50=0.559 msec                   
GET: 85077.42 requests per second, p50=0.551 msec                   


[1]  + 118716 done       redis-benchmark -h  -p 6379 -n 1000000 -r 100 --cluster -t set,get -q
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache-lab git:(master) ✗ 
➜   redis-benchmark -h elr10jemqn7ni0c5.vrpjxs.clustercfg.apse1.cache.amazonaws.com -p 6379 -n 2000000 -r 10 -P 50 --cluster -t get -q &
[1] 119067
Cluster has 2 master nodes:                                                                                                                                   

Master 0: 0decb1e57e0641b0deeb9a5c3396b0c92d05e5fa 172.31.47.24:6379
WARNING: Could not fetch node CONFIG 172.31.47.24:6379
Master 1: cd3c1cd4c41a25dad5a5084a79a930e6c057655f 172.31.19.44:6379
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache-lab git:(master) ✗ 
➜   WARNING: Could not fetch node CONFIG 172.31.19.44:6379

GET: 1996008.00 requests per second, p50=1.119 msec                     


[1]  + 119067 done       redis-benchmark -h  -p 6379 -n 2000000 -r 10 -P 50 --cluster -t get -q
admin@sin-bastion-0: /home/admin/myspace/github.com/zhang1980s/aws-note/elasticache-lab git:(master) ✗ 
➜   redis-cli -h elr10jemqn7ni0c5.vrpjxs.clustercfg.apse1.cache.amazonaws.com -p 6379 --hotkeys                                        

# Scanning the entire keyspace to find hot keys as well as
# average sizes per key type.  You can use -i 0.1 to sleep 0.1 sec
# per 100 SCAN commands (not usually needed).

100.00% ||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||
Keys sampled: 362388

-------- summary -------

hot key found with counter: 8   keyname: "key:{5ZA}:000000000077"
hot key found with counter: 8   keyname: "key:{2NM}:000000000021"
hot key found with counter: 8   keyname: "key:{r5}}:000000000090"
hot key found with counter: 8   keyname: "key:{6BD}:000000000022"
hot key found with counter: 8   keyname: "key:{a9}}:000000000025"
hot key found with counter: 8   keyname: "key:{vV}}:000000000007"
hot key found with counter: 8   keyname: "key:{267}:000000000005"
hot key found with counter: 8   keyname: "key:{22T}:000000000050"
hot key found with counter: 8   keyname: "key:{0Ml}:000000000030"
hot key found with counter: 7   keyname: "key:{5Od}:000000000009"
hot key found with counter: 7   keyname: "key:{4Lk}:000000000018"
hot key found with counter: 7   keyname: "key:{Qq}}:000000000083"
hot key found with counter: 7   keyname: "key:{eZ}}:000000000057"
hot key found with counter: 7   keyname: "key:{19U}:000000000090"
hot key found with counter: 7   keyname: "key:{8CF}:000000000066"
hot key found with counter: 7   keyname: "key:{Hk}}:000000000064"
```