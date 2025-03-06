import boto3
import json
import os
from datetime import datetime

def manage_capacity_reservation(instance_type, availability_zone_id, max_reservations, counter_file_base, log_file_base, reservation_tag):

    ec2 = boto3.client('ec2', region_name=region)

    # Construct the full counter_file and log_file paths
    counter_file = f"{counter_file_base}_{reservation_tag}.txt"
    log_file = f"{log_file_base}_{reservation_tag}.txt"

    def get_reservation_usage(reservation_tag):
        """Gets the capacity reservation usage for the specified instance type, AZ and reservation_tag."""
        try:
            response = ec2.describe_capacity_reservations(
                Filters=[
                    {
                        'Name': 'instance-type',
                        'Values': [instance_type]
                    },
                    {
                        'Name': 'availability-zone-id',
                        'Values': [availability_zone_id]
                    },
                    {
                        'Name': 'state',
                        'Values': ['active']
                    },
                    {
                        'Name': 'tag:AutoReservation',  
                        'Values': [reservation_tag]
                    }
                ]
            )
            total_reserved_count = 0
            available_instance_count = 0

            for reservation in response['CapacityReservations']:
                total_reserved_count += reservation['TotalInstanceCount']
                available_instance_count += reservation['AvailableInstanceCount']

            return total_reserved_count, available_instance_count

        except Exception as e:
            log_message(f"Error getting reservation usage: {e}")
            return 0, 0

    def create_reservation(reservation_tag):
        """Creates a new capacity reservation with reservation_tag."""
        try:
            response = ec2.create_capacity_reservation(
                InstanceType=instance_type,
                InstancePlatform='Linux/UNIX',
                AvailabilityZoneId=availability_zone_id,
                InstanceCount=1,
                EndDateType='unlimited',
                InstanceMatchCriteria='targeted',
                TagSpecifications=[
                    {
                        'ResourceType': 'capacity-reservation',
                        'Tags': [
                            {
                                'Key': 'AutoReservation',
                                'Value': reservation_tag
                            },
                        ]
                    },
                ]
            )
            log_message(f"Capacity reservation created: {json.dumps(response, indent=4, default=str)}")
            return response
        except Exception as e:
            log_message(f"Error creating capacity reservation: {e}")
            return None


    def get_reserved_count():
        try:
            with open(counter_file, 'r') as f:
                return int(f.read().strip())
        except FileNotFoundError:
            return 0
        except ValueError:
            log_message(f"Error: Invalid content in counter file {counter_file}.  Resetting to 0.")
            return 0

    def update_reserved_count(count):
        try:
            with open(counter_file, 'w') as f:
                f.write(str(count))
        except Exception as e:
            log_message(f"Error updating counter file: {e}")


    def log_message(message):
        timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        log_entry = f"[{timestamp}] {message}\n"
        print(log_entry, end='')  # Print to console
        try:
            with open(log_file, 'a') as f:
                f.write(log_entry)
        except Exception as e:
            print(f"Error writing to log file: {e}") # Log to console even if file write fails.



    log_message(f"Checking capacity reservations for {instance_type} in {availability_zone_id} with tag {reservation_tag}")

    total_reserved_count, available_instance_count = get_reservation_usage(reservation_tag)
    log_message(f"Total reserved instances: {total_reserved_count}, Available instances: {available_instance_count}")

    reserved_count_from_file = get_reserved_count()
    log_message(f"Reserved count from counter file: {reserved_count_from_file}")
    if total_reserved_count > reserved_count_from_file:
        update_reserved_count(total_reserved_count)
        reserved_count_from_file = total_reserved_count
        log_message(f"Update counter file to : {reserved_count_from_file} by actual reservation number.")
    if available_instance_count < max_reservations:
        reservations_needed = max_reservations - available_instance_count

        for _ in range(reservations_needed):
            log_message(f"Available instances are less than maximum ({max_reservations}). Creating a reservation...")
            response = create_reservation(reservation_tag)
            if response:
              reserved_count_from_file += 1
              update_reserved_count(reserved_count_from_file)
            else:
                break
    else:
        log_message(f"Sufficient capacity available. No new reservations needed.")

    log_message("Capacity reservation check complete.")


if __name__ == "__main__":
    region = 'ap-east-1'
    instance_type = "m6i.8xlarge"
    availability_zone_id = "ape1-az1"
    max_reservations = 10
    counter_file_base = "/home/admin/tmp/reservation_count"  
    log_file_base = "/home/admin/tmp/capacity_reservation_log" 
    reservation_tag = "my-capacity-reservation"
    manage_capacity_reservation(instance_type, availability_zone_id, max_reservations, counter_file_base, log_file_base, reservation_tag)