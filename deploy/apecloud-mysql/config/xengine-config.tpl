[mysqld]
## https://infracreate.feishu.cn/wiki/wikcnkakA8j6q5Qxp4ckeFzOtGf

{{- $data_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- $phy_cpu := getContainerCPU ( index $.podSpec.containers 0 ) }}

## xengine base config
default_storage_engine=xengine
default_tmp_storage_engine=innodb

# log_error_verbosity=3
binlog-format=row

## non classes config

xengine_datadir={{ $data_root }}/xengine
xengine_wal_dir={{ $data_root }}/xengine
xengine_flush_log_at_trx_commit=1
xengine_enable_2pc=1
xengine_batch_group_slot_array_size=5
xengine_batch_group_max_group_size=15
xengine_batch_group_max_leader_wait_time_us=50
xengine_block_size=16384
xengine_disable_auto_compactions=0
xengine_dump_memtable_limit_size=0

xengine_min_write_buffer_number_to_merge=1
xengine_level0_file_num_compaction_trigger=64
xengine_level0_layer_num_compaction_trigger=2
xengine_level1_extent s_major_compaction_trigger=1000
xengine_level2_usage_percent=70
xengine_flush_delete_percent=70
xengine_compaction_delete_percent=50
xengine_flush_delete_percent_trigger=700000
xengine_flush_delete_record_trigger=700000
xengine_scan_add_blocks_limit=100

xengine_compression_per_level=kZSTD:KZSTD:kZSTD


## classes classes config

{{- if gt $phy_memory 0 }}
{{- $phy_memory := div $phy_memory ( mul 1024 1024 ) }}
xengine_write_buffer_size={{ min ( max 32 ( mulf $phy_memory 0.01 ) ) 256 | int }}
xengine_db_write_buffer_size={{ mulf $phy_memory 0.3 | int }}
xengine_db_total_write_buffer_size={{ mulf $phy_memory 0.3 | int }}
xengine_block_cache_size={{ mulf $phy_memory 0.3 | int }}
xengine_row_cache_size={{ mulf $phy_memory 0.1 | int }}
xengine_max_total_wal_size={{ min ( mulf $phy_memory 0.3 ) ( mul 12 1024 ) | int }}
{{- end }}

{{- if gt $phy_cpu 0 }}
xengine_max_background_flushes={{ max 1 ( min ( div $phy_cpu 2 ) 8 ) | int }}
xengine_base_background_compactions={{ max 1 ( min ( div $phy_cpu 2 ) 8 ) | int }}
xengine_max_background_compactions={{ max 1 (min ( div $phy_cpu 2 ) 12 ) | int }}
{{- end }}

