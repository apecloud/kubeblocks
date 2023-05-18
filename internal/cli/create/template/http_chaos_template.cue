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

// required, command line input options for parameters and flags
options: {
	namespace: string
	selector: {}
	mode:     string
	value:    string
	duration: string

	target: string
	port:   int32
	path:   string
	method: string
	code?:  int32

	abort?: bool
	delay?: string

	repalceBody?:   bytes
	replacePath?:   string
	replaceMethod?: string

	patchBodyValue?: string
	patchBodyType?:  string
}

// required, k8s api resource content
content: {
	kind:       "HTTPChaos"
	apiVersion: "chaos-mesh.org/v1alpha1"
	metadata: {
		generateName: "http-chaos-"
		namespace:    options.namespace
	}
	spec: {
		selector: options.selector
		mode:     options.mode
		value:    options.value
		duration: options.duration

		target: options.target
		port:   options.port
		path:   options.path
		method: options.method
		if options.code != _|_ {
			code: options.code
		}

		if options.abort != _|_ {
			abort: options.abort
		}
		if options.delay != _|_ {
			delay: options.delay
		}
		if options.replaceBody != _|_ || options.replacePath != _|_ || options.replaceMethod != _|_ {
			replace: {
				if options.replaceBody != _|_ {
					body: options.replaceBody
				}
				if options.replacePath != _|_ {
					path: options.replacePath
				}
				if options.replaceMethod != _|_ {
					method: options.replaceMethod
				}
			}
		}
		if options.patchBodyValue != _|_ && options.patchBodyType != _|_ {
			patch: {
				if options.patchBodyValue != _|_ && options.patchBodyType != _|_ {
					body: {
						value: options.patchBodyValue
						type:  options.patchBodyType
					}
				}
			}
		}
	}
}
