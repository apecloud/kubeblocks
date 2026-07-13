/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package service

import (
	"encoding/json"
)

// maxEventMessageLength is the Kubernetes limit for the Event message field;
// creating an Event with a longer message is rejected by the apiserver.
const maxEventMessageLength = 1024

// marshalEventWithSizeLimit marshals an event and, when the encoded form
// exceeds the Event message limit, shrinks the event's free-text fields —
// message first (keeping its head, where the root error lives), then output —
// so that the payload stays valid JSON that event consumers can unmarshal.
// The message and output pointers must reference fields of the event being
// marshaled.
func marshalEventWithSizeLimit(event any, message *string, output *[]byte) ([]byte, error) {
	const marker = "...(truncated)"
	for {
		msg, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		if len(msg) <= maxEventMessageLength {
			return msg, nil
		}
		overflow := len(msg) - maxEventMessageLength
		switch {
		case len(*message) > len(marker):
			cut := min(overflow+len(marker), len(*message))
			*message = (*message)[:len(*message)-cut] + marker
		case len(*output) > 0:
			*output = (*output)[:len(*output)-min(overflow, len(*output))]
		default:
			// nothing left to shrink; send as-is and let the apiserver decide
			return msg, nil
		}
	}
}
