package tpcc

import (
	"context"
)

const stockLevelCount = `SELECT /*+ TIDB_INLJ(order_line,stock) */ COUNT(DISTINCT (s_i_id)) stock_count FROM order_line, stock 
WHERE ol_w_id = ? AND ol_d_id = ? AND ol_o_id < ? AND ol_o_id >= ? - 20 AND s_w_id = ? AND s_i_id = ol_i_id AND s_quantity < ?`
const stockLevelSelectDistrict = `SELECT d_next_o_id FROM district WHERE d_w_id = ? AND d_id = ?`

func (w *Workloader) runStockLevel(ctx context.Context, thread int) error {
	s := getTPCCState(ctx)

	tx, err := w.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	wID := randInt(s.R, 1, w.cfg.Warehouses)
	dID := randInt(s.R, 1, 10)
	threshold := randInt(s.R, 10, 20)

	// SELECT d_next_o_id INTO :o_id FROM district WHERE d_w_id=:w_id AND d_id=:d_id;

	var oID int
	if err := s.stockLevelStmt[stockLevelSelectDistrict].QueryRowContext(ctx, wID, dID).Scan(&oID); err != nil {
		return err
	}

	// SELECT COUNT(DISTINCT (s_i_id)) INTO :stock_count FROM order_line, stock
	// WHERE ol_w_id=:w_id AND ol_d_id=:d_id AND ol_o_id<:o_id AND ol_o_id>=:o_id-20
	// AND s_w_id=:w_id AND s_i_id=ol_i_id AND s_quantity < :threshold;
	var stockCount int
	if err := s.stockLevelStmt[stockLevelCount].QueryRowContext(ctx, wID, dID, oID, oID, wID, threshold).Scan(&stockCount); err != nil {
		return err
	}

	return tx.Commit()
}
