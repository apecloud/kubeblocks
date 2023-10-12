// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

parameters: {
	endpoint: *"http://loki-gateway/loki/api/v1/push" | string
	sending_queue: {
		enabled: *true | bool
		num_consumers: *3 | int
		queue_size: *128 | int
	}
	retry_on_failure: {
		enabled: *true | bool
		initial_interval: *"10s" | string
		max_interval: *"60s" | string
		max_elapsed_time: *"300s" | string
	}

}

output:
  loki: {
  	endpoint: parameters.endpoint
    sending_queue: {
    	enabled: parameters.sending_queue.enabled
      num_consumers: parameters.sending_queue.num_consumers
      queue_size: parameters.sending_queue.queue_size
    }
    retry_on_failure: {
    	enabled: parameters.retry_on_failure.enabled
      initial_interval: parameters.retry_on_failure.initial_interval
      max_interval: parameters.retry_on_failure.max_interval
      max_elapsed_time: parameters.retry_on_failure.max_elapsed_time
    }

  }



