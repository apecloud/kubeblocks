package tpcc

import (
	"context"
	"fmt"
	"time"
)

const (
	paymentUpdateDistrict           = `UPDATE district SET d_ytd = d_ytd + ? WHERE d_w_id = ? AND d_id = ?`
	paymentSelectDistrict           = `SELECT d_street_1, d_street_2, d_city, d_state, d_zip, d_name FROM district WHERE d_w_id = ? AND d_id = ?`
	paymentUpdateWarehouse          = `UPDATE warehouse SET w_ytd = w_ytd + ? WHERE w_id = ?`
	paymentSelectWarehouse          = `SELECT w_street_1, w_street_2, w_city, w_state, w_zip, w_name FROM warehouse WHERE w_id = ?`
	paymentSelectCustomerListByLast = `SELECT c_id FROM customer WHERE c_w_id = ? AND c_d_id = ? AND c_last = ? ORDER BY c_first`
	paymentSelectCustomerForUpdate  = `SELECT c_first, c_middle, c_last, c_street_1, c_street_2, c_city, c_state, c_zip, c_phone,
c_credit, c_credit_lim, c_discount, c_balance, c_since FROM customer WHERE c_w_id = ? AND c_d_id = ? 
AND c_id = ? FOR UPDATE`
	paymentUpdateCustomer = `UPDATE customer SET c_balance = c_balance - ?, c_ytd_payment = c_ytd_payment + ?, 
c_payment_cnt = c_payment_cnt + 1 WHERE c_w_id = ? AND c_d_id = ? AND c_id = ?`
	paymentSelectCustomerData     = `SELECT c_data FROM customer WHERE c_w_id = ? AND c_d_id = ? AND c_id = ?`
	paymentUpdateCustomerWithData = `UPDATE customer SET c_balance = c_balance - ?, c_ytd_payment = c_ytd_payment + ?, 
c_payment_cnt = c_payment_cnt + 1, c_data = ? WHERE c_w_id = ? AND c_d_id = ? AND c_id = ?`
	paymentInsertHistory = `INSERT INTO history (h_c_d_id, h_c_w_id, h_c_id, h_d_id, h_w_id, h_date, h_amount, h_data)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
)

type paymentData struct {
	wID     int
	dID     int
	cWID    int
	cDID    int
	hAmount float64

	wStreet1 string
	wStreet2 string
	wCity    string
	wState   string
	wZip     string
	wName    string

	dStreet1 string
	dStreet2 string
	dCity    string
	dState   string
	dZip     string
	dName    string

	cID        int
	cFirst     string
	cMiddle    string
	cLast      string
	cStreet1   string
	cStreet2   string
	cCity      string
	cState     string
	cZip       string
	cPhone     string
	cSince     string
	cCredit    string
	cCreditLim float64
	cDiscount  float64
	cBalance   float64
	cData      string
}

func (w *Workloader) runPayment(ctx context.Context, thread int) error {
	s := getTPCCState(ctx)

	d := paymentData{
		wID:     randInt(s.R, 1, w.cfg.Warehouses),
		dID:     randInt(s.R, 1, districtPerWarehouse),
		hAmount: float64(randInt(s.R, 100, 500000)) / float64(100.0),
	}

	// Refer 2.5.1.2, 60% by last name, 40% by customer ID
	if s.R.Intn(100) < 60 {
		d.cLast = randCLast(s.R, s.Buf)
	} else {
		d.cID = randCustomerID(s.R)
	}

	// Refer 2.5.1.2, 85% by local, 15% by remote
	if w.cfg.Warehouses == 1 || s.R.Intn(100) < 85 {
		d.cWID = d.wID
		d.cDID = d.dID
	} else {
		d.cWID = w.otherWarehouse(ctx, d.wID)
		d.cDID = randInt(s.R, 1, districtPerWarehouse)
	}

	tx, err := w.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Process 1
	if _, err := s.paymentStmts[paymentUpdateDistrict].ExecContext(ctx, d.hAmount, d.wID, d.dID); err != nil {
		return fmt.Errorf("exec %s failed %v", paymentUpdateDistrict, err)
	}

	// Process 2
	if err := s.paymentStmts[paymentSelectDistrict].QueryRowContext(ctx, d.wID, d.dID).Scan(&d.dStreet1, &d.dStreet2,
		&d.dCity, &d.dState, &d.dZip, &d.dName); err != nil {
		return fmt.Errorf("exec %s failed %v", paymentSelectDistrict, err)
	}

	// Process 3
	if _, err := s.paymentStmts[paymentUpdateWarehouse].ExecContext(ctx, d.hAmount, d.wID); err != nil {
		return fmt.Errorf("exec %s failed %v", paymentUpdateWarehouse, err)
	}

	// Process 4
	if err := s.paymentStmts[paymentSelectWarehouse].QueryRowContext(ctx, d.wID).Scan(&d.wStreet1, &d.wStreet2,
		&d.wCity, &d.wState, &d.wZip, &d.wName); err != nil {
		return fmt.Errorf("exec %s failed %v", paymentSelectDistrict, err)
	}

	if d.cID == 0 {
		// Process 5
		rows, err := s.paymentStmts[paymentSelectCustomerListByLast].QueryContext(ctx, d.cWID, d.cDID, d.cLast)
		if err != nil {
			return fmt.Errorf("exec %s failed %v", paymentSelectCustomerListByLast, err)
		}
		var ids []int
		for rows.Next() {
			var id int
			if err = rows.Scan(&id); err != nil {
				return fmt.Errorf("exec %s failed %v", paymentSelectCustomerListByLast, err)
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return fmt.Errorf("customer for (%d, %d, %s) not found", d.cWID, d.cDID, d.cLast)
		}
		d.cID = ids[(len(ids)+1)/2-1]
	}

	// Process 6
	if err := s.paymentStmts[paymentSelectCustomerForUpdate].QueryRowContext(ctx, d.cWID, d.cDID, d.cID).Scan(&d.cFirst, &d.cMiddle, &d.cLast,
		&d.cStreet1, &d.cStreet2, &d.cCity, &d.cState, &d.cZip, &d.cPhone, &d.cCredit, &d.cCreditLim,
		&d.cDiscount, &d.cBalance, &d.cSince); err != nil {
		return fmt.Errorf("exec %s failed %v", paymentSelectCustomerForUpdate, err)
	}

	if d.cCredit == "BC" {
		// Process 7
		if err := s.paymentStmts[paymentSelectCustomerData].QueryRowContext(ctx, d.cWID, d.cDID, d.cID).Scan(&d.cData); err != nil {
			return fmt.Errorf("exec %s failed %v", paymentSelectCustomerData, err)
		}

		newData := fmt.Sprintf("| %4d %2d %4d %2d %4d $%7.2f %12s %24s", d.cID, d.cDID, d.cWID,
			d.dID, d.wID, d.hAmount, time.Now().Format(timeFormat), d.cData)
		if len(newData) >= 500 {
			newData = newData[0:500]
		} else {
			newData += d.cData[0 : 500-len(newData)]
		}

		// Process 8
		if _, err := s.paymentStmts[paymentUpdateCustomerWithData].ExecContext(ctx, d.hAmount, d.hAmount, newData, d.cWID, d.cDID, d.cID); err != nil {
			return fmt.Errorf("exec %s failed %v", paymentUpdateCustomerWithData, err)
		}
	} else {
		// Process 9
		if _, err := s.paymentStmts[paymentUpdateCustomer].ExecContext(ctx, d.hAmount, d.hAmount, d.cWID, d.cDID, d.cID); err != nil {
			return fmt.Errorf("exec %s failed %v", paymentUpdateCustomer, err)
		}
	}

	// Process 10
	hData := fmt.Sprintf("%10s    %10s", d.wName, d.dName)
	if _, err := s.paymentStmts[paymentInsertHistory].ExecContext(ctx, d.cDID, d.cWID, d.cID, d.dID, d.wID, time.Now().Format(timeFormat), d.hAmount, hData); err != nil {
		return fmt.Errorf("exec %s failed %v", paymentInsertHistory, err)
	}

	return tx.Commit()
}
