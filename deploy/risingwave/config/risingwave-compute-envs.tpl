RW_PARALLELISM: "{{ getContainerCPU ( index $.podSpec.containers 0 ) }}"
RW_TOTAL_MEMORY_BYTES: "{{ getContainerMemory ( index $.podSpec.containers 0 ) }}"