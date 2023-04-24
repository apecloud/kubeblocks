/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
