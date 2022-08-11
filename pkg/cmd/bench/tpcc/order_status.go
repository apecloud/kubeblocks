package tpcc

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	orderStatusSelectCustomerCntByLast = `SELECT count(c_id) namecnt FROM customer WHERE c_w_id = ? AND c_d_id = ? AND c_last = ?`
	orderStatusSelectCustomerByLast    = `SELECT c_balance, c_first, c_middle, c_id FROM customer WHERE c_w_id = ? AND c_d_id = ? AND c_last = ? ORDER BY c_first`
	orderStatusSelectCustomerByID      = `SELECT c_balance, c_first, c_middle, c_last FROM customer WHERE c_w_id = ? AND c_d_id = ? AND c_id = ?`
	orderStatusSelectLatestOrder       = `SELECT o_id, o_carrier_id, o_entry_d FROM orders WHERE o_w_id = ? AND o_d_id = ? AND o_c_id = ? ORDER BY o_id DESC LIMIT 1`
	orderStatusSelectOrderLine         = `SELECT ol_i_id, ol_supply_w_id, ol_quantity, ol_amount, ol_delivery_d FROM order_line WHERE ol_w_id = ? AND ol_d_id = ? AND ol_o_id = ?`
)

type orderStatusData struct {
	wID int
	dID int

	cID      int
	cLast    string
	cBalance float64
	cFirst   string
	cMiddle  string

	oID        int
	oEntryD    string
	oCarrierID sql.NullInt64
}

func (w *Workloader) runOrderStatus(ctx context.Context, thread int) error {
	s := getTPCCState(ctx)
	d := orderStatusData{
		wID: randInt(s.R, 1, w.cfg.Warehouses),
		dID: randInt(s.R, 1, districtPerWarehouse),
	}

	// refer 2.6.1.2
	if s.R.Intn(100) < 60 {
		d.cLast = randCLast(s.R, s.Buf)
	} else {
		d.cID = randCustomerID(s.R)
	}

	tx, err := w.beginTx(ctx)
	if err != nil {
		return err
	}
	//nolint
	defer tx.Rollback()

	if d.cID == 0 {
		// by name
		// SELECT count(c_id) INTO :namecnt FROM customer
		//	WHERE c_last=:c_last AND c_d_id=:d_id AND c_w_id=:w_id
		var nameCnt int
		if err := s.orderStatusStmts[orderStatusSelectCustomerCntByLast].QueryRowContext(ctx, d.wID, d.dID, d.cLast).Scan(&nameCnt); err != nil {
			return fmt.Errorf("exec %s failed %v", orderStatusSelectCustomerCntByLast, err)
		}
		if nameCnt%2 == 1 {
			nameCnt++
		}

		rows, err := s.orderStatusStmts[orderStatusSelectCustomerByLast].QueryContext(ctx, d.wID, d.dID, d.cLast)
		if err != nil {
			return fmt.Errorf("exec %s failed %v", orderStatusSelectCustomerByLast, err)
		}
		for i := 0; i < nameCnt/2 && rows.Next(); i++ {
			if err := rows.Scan(&d.cBalance, &d.cFirst, &d.cMiddle, &d.cID); err != nil {
				return err
			}
		}

		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	} else {
		if err := s.orderStatusStmts[orderStatusSelectCustomerByID].QueryRowContext(ctx, d.wID, d.dID, d.cID).Scan(&d.cBalance, &d.cFirst, &d.cMiddle, &d.cLast); err != nil {
			return fmt.Errorf("exec %s failed %v", orderStatusSelectCustomerByID, err)
		}
	}

	// SELECT o_id, o_carrier_id, o_entry_d
	//  INTO :o_id, :o_carrier_id, :entdate FROM orders
	//  ORDER BY o_id DESC;

	// refer 2.6.2.2 - select the latest order
	if err := s.orderStatusStmts[orderStatusSelectLatestOrder].QueryRowContext(ctx, d.wID, d.dID, d.cID).Scan(&d.oID, &d.oCarrierID, &d.oEntryD); err != nil {
		return fmt.Errorf("exec %s failed %v", orderStatusSelectLatestOrder, err)
	}

	// SQL DECLARE c_line CURSOR FOR SELECT ol_i_id, ol_supply_w_id, ol_quantity,
	// 	ol_amount, ol_delivery_d
	// 	FROM order_line
	// 	WHERE ol_o_id=:o_id AND ol_d_id=:d_id AND ol_w_id=:w_id;
	// OPEN c_line;
	rows, err := s.orderStatusStmts[orderStatusSelectOrderLine].QueryContext(ctx, d.wID, d.dID, d.oID)
	if err != nil {
		return fmt.Errorf("exec %s failed %v", orderStatusSelectOrderLine, err)
	}
	defer rows.Close()

	//items := make([]orderItem, 0, 4)
	for rows.Next() {
		var item orderItem
		if err := rows.Scan(&item.olIID, &item.olSupplyWID, &item.olQuantity, &item.olAmount, &item.olDeliveryD); err != nil {
			return err
		}
		//items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return tx.Commit()
}
