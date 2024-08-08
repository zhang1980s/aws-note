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