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

package apps

import (
	"context"

	"github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/testutil"
)

type ObjectCreator struct {
	client.Object
}

func (c *ObjectCreator) Apply(changeFn func(object client.Object)) *ObjectCreator {
	changeFn(c.Object)
	return c
}

func (c *ObjectCreator) Create(testCtx *testutil.TestContext) *ObjectCreator {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, c.Object)).Should(gomega.Succeed())
	return c
}

func (c *ObjectCreator) CheckedCreate(testCtx *testutil.TestContext) *ObjectCreator {
	gomega.Expect(testCtx.CheckedCreateObj(testCtx.Ctx, c.Object)).Should(gomega.Succeed())
	return c
}

func (c *ObjectCreator) CreateCli(ctx context.Context, cli client.Client) *ObjectCreator {
	gomega.Expect(cli.Create(ctx, c.Object)).Should(gomega.Succeed())
	return c
}

func (c *ObjectCreator) GetObject() client.Object {
	return c.Object
}
