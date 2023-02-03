/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/rand"
)

// use plants common name that length does not greater than 6 characters
// reference https://en.wikipedia.org/wiki/List_of_plants_by_common_name
var names = [...]string{
	"alder",
	"almond",
	"aloe",
	"apple",
	"azolla",
	"bamboo",
	"banana",
	"baobab",
	"bean",
	"beech",
	"birch",
	"brier",
	"carrot",
	"cedar",
	"cherry",
	"clove",
	"clover",
	"cornel",
	"cress",
	"daisy",
	"durian",
	"fennel",
	"ferns",
	"fig",
	"flax",
	"garlic",
	"holly",
	"ivy",
	"laurel",
	"leek",
	"lemon",
	"lilac",
	"lupin",
	"maize",
	"mango",
	"maple",
	"oak",
	"olive",
	"onion",
	"orange",
	"osier",
	"pea",
	"peach",
	"peanut",
	"pear",
	"pine",
	"poplar",
	"redbud",
	"rice",
	"rose",
	"rye",
	"tansy",
	"tea",
	"tulip",
	"weed",
	"wheat",
	"willow",
	"yam",
}

func GenerateName() string {
	return fmt.Sprintf("%s%02d", names[rand.Intn(len(names))], rand.IntnRange(1, 100))
}
