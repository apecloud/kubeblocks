package tpcc

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type deliveryData struct {
	wID         int
	oCarrierID  int
	olDeliveryD string
}

const (
	deliverySelectNewOrder = "SELECT no_o_id FROM new_order WHERE no_w_id = ? AND no_d_id = ? ORDER BY no_o_id ASC LIMIT 1 FOR UPDATE"
	deliveryDeleteNewOrder = `DELETE FROM new_order WHERE (no_w_id, no_d_id, no_o_id) IN (
	(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?)
)`
	deliveryUpdateOrder = `UPDATE orders SET o_carrier_id = ? WHERE (o_w_id, o_d_id, o_id) IN (
	(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?)
)`
	deliverySelectOrders = `SELECT o_d_id, o_c_id FROM orders WHERE (o_w_id, o_d_id, o_id) IN (
	(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?)
)`
	deliveryUpdateOrderLine = `UPDATE order_line SET ol_delivery_d = ? WHERE (ol_w_id, ol_d_id, ol_o_id) IN (
	(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?)
)`
	deliverySelectSumAmount = `SELECT ol_d_id, SUM(ol_amount) FROM order_line WHERE (ol_w_id, ol_d_id, ol_o_id) IN (
	(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?),(?,?,?)
) GROUP BY ol_d_id`
	deliveryUpdateCustomer = `UPDATE customer SET c_balance = c_balance + ?, c_delivery_cnt = c_delivery_cnt + 1 WHERE c_w_id = ? AND c_d_id = ? AND c_id = ?`
)

func (w *Workloader) runDelivery(ctx context.Context, thread int) error {
	s := getTPCCState(ctx)

	d := deliveryData{
		wID:        randInt(s.R, 1, w.cfg.Warehouses),
		oCarrierID: randInt(s.R, 1, 10),
	}

	tx, err := w.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	type deliveryOrder struct {
		oID    int
		cID    int
		amount float64
	}
	orders := make([]deliveryOrder, 10)
	for i := 0; i < districtPerWarehouse; i++ {
		if err = s.deliveryStmts[deliverySelectNewOrder].QueryRowContext(ctx, d.wID, i+1).Scan(&orders[i].oID); err == sql.ErrNoRows {
			continue
		} else if err != nil {
			return fmt.Errorf("exec %s failed %v", deliverySelectNewOrder, err)
		}
	}

	if _, err = s.deliveryStmts[deliveryDeleteNewOrder].ExecContext(ctx,
		d.wID, 1, orders[0].oID,
		d.wID, 2, orders[1].oID,
		d.wID, 3, orders[2].oID,
		d.wID, 4, orders[3].oID,
		d.wID, 5, orders[4].oID,
		d.wID, 6, orders[5].oID,
		d.wID, 7, orders[6].oID,
		d.wID, 8, orders[7].oID,
		d.wID, 9, orders[8].oID,
		d.wID, 10, orders[9].oID,
	); err != nil {
		return fmt.Errorf("exec %s failed %v", deliveryDeleteNewOrder, err)
	}

	if _, err = s.deliveryStmts[deliveryUpdateOrder].ExecContext(ctx, d.oCarrierID,
		d.wID, 1, orders[0].oID,
		d.wID, 2, orders[1].oID,
		d.wID, 3, orders[2].oID,
		d.wID, 4, orders[3].oID,
		d.wID, 5, orders[4].oID,
		d.wID, 6, orders[5].oID,
		d.wID, 7, orders[6].oID,
		d.wID, 8, orders[7].oID,
		d.wID, 9, orders[8].oID,
		d.wID, 10, orders[9].oID,
	); err != nil {
		return fmt.Errorf("exec %s failed %v", deliveryUpdateOrder, err)
	}

	if rows, err := s.deliveryStmts[deliverySelectOrders].QueryContext(ctx,
		d.wID, 1, orders[0].oID,
		d.wID, 2, orders[1].oID,
		d.wID, 3, orders[2].oID,
		d.wID, 4, orders[3].oID,
		d.wID, 5, orders[4].oID,
		d.wID, 6, orders[5].oID,
		d.wID, 7, orders[6].oID,
		d.wID, 8, orders[7].oID,
		d.wID, 9, orders[8].oID,
		d.wID, 10, orders[9].oID,
	); err != nil {
		return fmt.Errorf("exec %s failed %v", deliverySelectOrders, err)
	} else {
		for rows.Next() {
			var dID, cID int
			if err = rows.Scan(&dID, &cID); err != nil {
				return fmt.Errorf("exec %s failed %v", deliverySelectOrders, err)
			}
			orders[dID-1].cID = cID
		}
	}

	if _, err = s.deliveryStmts[deliveryUpdateOrderLine].ExecContext(ctx, time.Now().Format(timeFormat),
		d.wID, 1, orders[0].oID,
		d.wID, 2, orders[1].oID,
		d.wID, 3, orders[2].oID,
		d.wID, 4, orders[3].oID,
		d.wID, 5, orders[4].oID,
		d.wID, 6, orders[5].oID,
		d.wID, 7, orders[6].oID,
		d.wID, 8, orders[7].oID,
		d.wID, 9, orders[8].oID,
		d.wID, 10, orders[9].oID,
	); err != nil {
		return fmt.Errorf("exec %s failed %v", deliveryUpdateOrderLine, err)
	}

	if rows, err := s.deliveryStmts[deliverySelectSumAmount].QueryContext(ctx,
		d.wID, 1, orders[0].oID,
		d.wID, 2, orders[1].oID,
		d.wID, 3, orders[2].oID,
		d.wID, 4, orders[3].oID,
		d.wID, 5, orders[4].oID,
		d.wID, 6, orders[5].oID,
		d.wID, 7, orders[6].oID,
		d.wID, 8, orders[7].oID,
		d.wID, 9, orders[8].oID,
		d.wID, 10, orders[9].oID,
	); err != nil {
		return fmt.Errorf("exec %s failed %v", deliverySelectSumAmount, err)
	} else {
		for rows.Next() {
			var dID int
			var amount float64
			if err = rows.Scan(&dID, &amount); err != nil {
				return fmt.Errorf("exec %s failed %v", deliverySelectOrders, err)
			}
			orders[dID-1].amount = amount
		}
	}

	for i := 0; i < districtPerWarehouse; i++ {
		order := &orders[i]
		if order.oID == 0 {
			continue
		}
		if _, err = s.deliveryStmts[deliveryUpdateCustomer].ExecContext(ctx, order.amount, d.wID, i+1, order.cID); err != nil {
			return fmt.Errorf("exec %s failed %v", deliveryUpdateCustomer, err)
		}
	}
	return tx.Commit()
}
