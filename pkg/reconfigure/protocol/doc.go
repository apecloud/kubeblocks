/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// Package protocol defines the durable reconfigure execution protocol.
//
// FenceState values are immutable snapshots. Persisting a returned state must
// use resourceVersion compare-and-swap against the exact fence object read to
// construct the input state. The package does not merge concurrent snapshots;
// callers must reread and retry after a conflict.
//
// Consumed registrations and effects are durable tombstones and intentionally
// continue to count against RegistryLimits. A controller must rotate to a new
// installation fence before capacity is exhausted; this package does not
// compact or reclaim tombstones in place.
package protocol
