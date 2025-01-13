LOG=./track.log

>$LOG

RES=./res.txt

# unit is ns
MIN_OFFSET=20000

ETH=`ifconfig | grep mtu | grep -v "lo:" \
    | awk -F ':' '{ print $1;}' `;

SLOT=`cat /sys/class/net/$ETH/device/uevent \
	| grep PCI_SLOT_NAME | awk -F '=' '{ print $2;}'`;

while true
do
    CLOCK_ERROR_BOUND=`cat /sys/bus/pci/devices/$SLOT/phc_error_bound`;

    OFFSET=`echo "$CLOCK_ERROR_BOUND" | awk '{ if ( $1 > '"$MIN_OFFSET"' \
	       	|| $1 < -'"$MIN_OFFSET"' ) { print $1; } }'`;

    if [ "$OFFSET" != ""  ]
    then
        NOW=`date "+%Y%m%d %H:%M:%S.%6N"`;
        echo "WARN: $NOW offset is $OFFSET ,  res is $CLOCK_ERROR_BOUND ns "  >> $LOG

    fi

    sleep 1

done