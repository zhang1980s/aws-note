SELECT
    year,
    month,
    bill_payer_account_id,
    line_item_usage_account_id,
    product_servicename,
    product_region,
    product_instance_type_family,
    product_instance_type,
    product_physical_processor,
    cpu_type,
    sum(usage_hours) as sum_instance_monthly,
    sum(usage_hours * vcpu) as sum_vcpu_monthly
FROM
    (
        SELECT
            year,
            month,
            bill_payer_account_id,
            line_item_usage_account_id,
            product_servicename,
            product_region,
            product_instance_type_family,
            product_instance_type,
            product_physical_processor,
            product_current_generation,
            case
                when product_physical_processor like 'AMD%' then 'AMD'
                when product_physical_processor like 'AWS%' then 'Graviton'
                when product_physical_processor like 'Intel%' then 'Intel'
                when product_physical_processor like 'Apple%' then 'Apple'
                when product_product_name = 'Amazon ElastiCache'
                and (
                    product_instance_type like 'cache.m5%'
                    or product_instance_type like 'cache.t3%'
                    or product_instance_type like 'cache.t2%'
                    or product_instance_type like 'cache.m4%'
                    or product_instance_type like 'cache.r5%'
                    or product_instance_type like 'cache.r4%'
                ) then 'Intel'
                when product_product_name = 'Amazon ElastiCache'
                and (
                    product_instance_type like 'cache.t4g%'
                    or product_instance_type like 'cache.m7g%'
                    or product_instance_type like 'cache.m6g%'
                    or product_instance_type like 'cache.r7g%'
                    or product_instance_type like 'cache.r6g%'
                    or product_instance_type like 'cache.c7g%'
                ) then 'Graviton'
                when product_product_name = 'Amazon MemoryDB' then 'Graviton'
                when product_product_name = 'Amazon Managed Streaming for Apache Kafka'
                and product_product_family = 'Managed Streaming for Apache Kafka (MSK)'
                and line_item_operation = 'RunBroker'
                and (
                    product_compute_family like 't3%'
                    or product_compute_family like 'm5%'
                ) then 'Intel'
                when product_product_name = 'Amazon Managed Streaming for Apache Kafka'
                and product_product_family = 'Managed Streaming for Apache Kafka (MSK)'
                and line_item_operation = 'RunBroker'
                and product_compute_family like 'm7g%' then 'Graviton'
                else 'FIXME'
            end as cpu_type,
            CAST(NULLIF("trim"(product_vcpu), '') AS integer) vcpu,
            sum(line_item_usage_amount) usage_hours
        FROM
            customer_cur_data.customer_all
        WHERE
            bill_invoicing_entity = 'Amazon Web Services, Inc.'
            and bill_bill_type = 'Anniversary'
            and pricing_unit in ('Hrs', 'hours')
            and product_servicename in (
                'Amazon Elastic Compute Cloud',
                'Amazon Relational Database Service',
                'Amazon ElastiCache',
                'Amazon MemoryDB',
                'Amazon Managed Streaming for Apache Kafka'
            )
            and product_product_family in (
                'Compute Instance',
                'Compute Instance (bare metal)',
                'Compute Instance (spot)',
                'Database Instance',
                'Cache Instance',
                'Amazon MemoryDB',
                'Managed Streaming for Apache Kafka (MSK)'
            )
            and year = '2024'
        GROUP BY
            1,
            2,
            3,
            4,
            5,
            6,
            7,
            8,
            9,
            10,
            11,
            12
    )
GROUP BY
    1,
    2,
    3,
    4,
    5,
    6,
    7,
    8,
    9,
    10