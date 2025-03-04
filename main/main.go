package main

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

var templateText = `
{{- $page_size := 8 }}
{{- $log_path :=  "log" }}
{{- $arch_path := "arch" }}
{{- $phy_memory := mul 16 1073741824 }}
{{- $phy_cpu_value := 8 }}
{{- $phy_cpu := 0 }}
{{- if gt $phy_cpu_value 0 }}
  {{- $phy_cpu = $phy_cpu_value | int }}
  {{- if eq $phy_cpu 0 }}
     {{- $phy_cpu = 1 }}
  {{- end }}
{{- end }}

{{- $v_mem_mb := mul (div $phy_memory 1073741824) 800 | int }}
{{- $v_mem_mb = mul (div $v_mem_mb 1000) 1000 | int  }}
{{- $magic_mem_mb := div (mul $v_mem_mb 625) 10000 | int }}
{{- if gt $v_mem_mb 512000 }}
  {{- $v_mem_mb = div (mul $v_mem_mb 8) 10 | int }}
{{- end }}

{{- $memory_target := div (mul $v_mem_mb 12) 100 | int }}
{{- $memory_target := mul (div $memory_target 1000) 1000 | int }}
{{- $task_threads := 4 }}
{{- $io_thr_groups := 4 }}
{{- if lt $phy_cpu 8 }}
  {{- $task_threads = 4 }}
  {{- $io_thr_groups = 2 }}
{{- end }}

{{- if ge $phy_cpu 64 }}
  {{- $phy_cpu = 64 }}
  {{- $task_threads = 16 }}
  {{- $io_thr_groups = 8 }}
{{- end }}

{{- $buffer := div (mul $v_mem_mb 4) 10 | int }}
{{- $buffer := mul (div $buffer 1000) 1000 | int }}
{{- $recycle := div (mul $v_mem_mb 4) 100 | int }}
{{- $buffer_pools := 101 }}
{{- if lt $v_mem_mb 70000 }}
  {{- $min_prime := div $v_mem_mb 800 | int }}
  {{- $rns := list 2 3 5 7 11 13 17 19 23 29 31 37 41 43 47 53 59 61 67 71 73 79 83 89 97 101 | toStrings }}
  {{- range $rn := $rns }}
	{{- $rn = $rn | int }}
    {{- if gt $rn $min_prime }}
      {{- $buffer_pools = $rn }}
      {{- break }}
    {{- end }}
  {{- end }}
{{- end }}

{{- $memory_pool := 500 }}
{{- $sort_buf_global_size := 1000 }}
{{- $memory_n_pools := 1 }}
{{- $cache_pool_size := 100 }}
{{- $fast_pool_pages := 3000 }}
{{- $fast_roll_pages := 1000 }}
{{- $sort_flag := 1 }}
{{- $sort_blk_size := 1 }}
{{- $sort_buf_size := 1 }}
{{- $rlog_pool_size := 256 }}
{{- $hj_buf_global_size := 5000 }}
{{- $hagr_buf_global_size := 5000 }}
{{- $hj_buf_size := 500 }}
{{- $hagr_buf_size := 500 }}
{{- $dict_buf_size := 50 }}

{{- if ge $v_mem_mb 16000 }}
  {{- if eq $v_mem_mb 16000 }}
    {{- $memory_pool = 1500 }}
    {{- $sort_buf_global_size = 1000 }}
    {{- $memory_n_pools = 3 }}
    {{- $cache_pool_size = 512 }}
  {{- else }}
    {{- $memory_pool = 2000 }}
    {{- $sort_buf_global_size = 2000 }}
    {{- $memory_n_pools = 11 }}
    {{- $cache_pool_size = 1024 }}
  {{- end }}

  {{- $fast_pool_pages = 9999 }}
  {{- $sort_flag = 0 }}
  {{- $sort_blk_size = 1 }}
  {{- $sort_buf_size = 10 }}
  {{- $rlog_pool_size = 1024 }}
  {{- $hj_buf_global_size = min $magic_mem_mb 10000 | int }}
  {{- $hagr_buf_global_size = min $magic_mem_mb 10000 | int }}
  {{- $hj_buf_size = 250 }}
  {{- $hagr_buf_size = 250 }}
  {{- if ge $v_mem_mb 64000 }}
    {{- $fast_pool_pages = 99999 }}
    {{- $fast_roll_pages = 9999 }}
    {{- $buffer = sub $buffer 3000 }}
    {{- $cache_pool_size = 2048 }}
    {{- $rlog_pool_size := 2048 }}
    {{- $sort_flag = 1 }}
    {{- $sort_blk_size = 1 }}
    {{- $sort_buf_size = 50 }}
    {{- $sort_buf_global_size = div (mul $v_mem_mb 2) 100 | int }}
    {{- $hj_buf_global_size = div (mul $v_mem_mb 15625) 100000 | int }}
    {{- $hagr_buf_global_size = div (mul $v_mem_mb 4) 100 | int }}
    {{- $hj_buf_size = 512 }}
    {{- $hagr_buf_size = 512 }}
    {{- $memory_n_pools = 59 }}
  {{- end }}

  {{- $dict_buf_size = 50 }}
  {{- $hj_buf_global_size = div (mul $hj_buf_global_size 1000) 1000 | int }}
  {{- $sort_buf_global_size = div (mul $sort_buf_global_size 1000) 1000 | int }}
  {{- $hagr_buf_global_size = div (mul $hagr_buf_global_size 1000) 1000 | int }}
  {{- $recycle = div (mul $recycle 1000) 1000 | int }}
{{- else }}
  {{- $memory_pool = max $magic_mem_mb 200 | int }}
  {{- $memory_pool = mul (div $memory_pool 100) 100 | int }}
  {{- $memory_n_pools = 2 }}
  {{- $cache_pool_size = 200 }}
  {{- $rlog_pool_size = 256 }}
  {{- $sort_buf_size = 10 }}
  {{- $sort_buf_global_size = 500 }}
  {{- $dict_buf_size = 50 }}
  {{- $sort_flag = 0 }}
  {{- $sort_blk_size = 1 }}
  {{- $hj_buf_global_size = max $magic_mem_mb 500 | int }}
  {{- $hagr_buf_global_size = max $magic_mem_mb 500 | int }}
  {{- $hj_buf_size = max (div $magic_mem_mb 10) 50 | int }}
  {{- $hagr_buf_size = max (div $magic_mem_mb 10) 50 | int }}
{{- end }}

{{- $max_buffer := $buffer }}
{{- $recycle_pools := 19 }}
{{- $page_size_magic := mul $page_size 3000 | int }}
{{- $min_prime := div (mul $recycle 1024) $page_size_magic | int }}
{{- $rns := list 2 3 5 7 11 13 17 19 23 29 31 37 41 43 47 53 59 61 67 71 73 79 83 89 97 101 | toStrings }}
{{- range $rn := $rns }}
{{- $rn = $rn | int }}
{{- if le $rn $min_prime }}
  {{- $recycle_pools = $rn }}
  {{- break }}
{{- end }}
{{- end }}

# set configuration
WORKER_THREADS = {{ $phy_cpu }}
IO_THR_GROUPS = {{ $io_thr_groups }}
GEN_SQL_MEM_RECLAIM = 0
MAX_OS_MEMORY = 100
MEMORY_POOL = {{ $memory_pool }}
MEMORY_N_POOLS = {{ $memory_n_pools }}
MEMORY_TARGET = {{ $memory_target }}
MEMORY_MAGIC_CHECK = 1
BUFFER = {{ $buffer }}
MAX_BUFFER = {{ $max_buffer }}
BUFFER_POOLS = {{ $buffer_pools }}
RECYCLE = {{ $recycle }}
RECYCLE_POOLS = {{ $recycle_pools }}
FAST_POOL_PAGES = {{ $fast_pool_pages }}
FAST_ROLL_PAGES = {{ $fast_roll_pages }}
TASK_THREADS = {{ $task_threads }}
ENABLE_FREQROOTS = 1
MULTI_PAGE_GET_NUM = 1
PRELOAD_SCAN_NUM = 0
PRELOAD_EXTENT_NUM = 0
HJ_BUF_GLOBAL_SIZE = {{ $hj_buf_global_size }}
HJ_BUF_SIZE = {{ $hj_buf_size }}
HAGR_BUF_GLOBAL_SIZE = {{ $hagr_buf_global_size }}
HAGR_BUF_SIZE = {{ $hagr_buf_size }}
SORT_FLAG = {{ $sort_flag }}
SORT_BLK_SIZE = {{ $sort_blk_size }}
SORT_BUF_SIZE = {{ $sort_buf_size }}
SORT_BUF_GLOBAL_SIZE = {{ $sort_buf_global_size }}
RLOG_POOL_SIZE = {{ $rlog_pool_size }}
CACHE_POOL_SIZE = {{ $cache_pool_size }}
DICT_BUF_SIZE = {{ $dict_buf_size }}
VM_POOL_TARGET = 16384
SESS_POOL_TARGET = 16384
USE_PLN_POOL = 1
ENABLE_MONITOR = 1
SVR_LOG = 0
TEMP_SIZE = 1024
TEMP_SPACE_LIMIT = 102400
MAX_SESSIONS = 1500
MAX_SESSION_STATEMENT = 20000
PK_WITH_CLUSTER = 1
ENABLE_ENCRYPT = 0
OLAP_FLAG = 2
VIEW_PULLUP_FLAG = 1
OPTIMIZER_MODE = 1
ADAPTIVE_NPLN_FLAG = 0
PARALLEL_PURGE_FLAG = 1
PARALLEL_POLICY = 2
UNDO_EXTENT_NUM = 16
ENABLE_INJECT_HINT = 1
FAST_LOGIN = 1
BTR_SPLIT_MODE = 1
ENABLE_MONITOR_BP = 0
`

func main() {
	tmpl, err := template.New("opsDefTemplate").Funcs(sprig.TxtFuncMap()).Parse(templateText)
	if err != nil {
		panic(err)
	}
	var buf strings.Builder
	if err = tmpl.Execute(&buf, nil); err != nil {
		panic(err)
	}
	fmt.Println(buf.String())
}
