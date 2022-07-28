package tpcc

import (
	"context"
	"fmt"
)

const (
	tableItem      = "item"
	tableCustomer  = "customer"
	tableDistrict  = "district"
	tableOrders    = "orders"
	tableNewOrder  = "new_order"
	tableOrderLine = "order_line"
	tableHistory   = "history"
	tableWareHouse = "warehouse"
	tableStock     = "stock"
)

type ddlManager struct {
	parts         int
	warehouses    int
	partitionType int
	useFK         bool
}

func newDDLManager(parts int, useFK bool, warehouses, partitionType int) *ddlManager {
	return &ddlManager{parts: parts, useFK: useFK, warehouses: warehouses, partitionType: partitionType}
}

func (w *ddlManager) createTableDDL(ctx context.Context, query string, tableName string) error {
	s := getTPCCState(ctx)
	fmt.Printf("creating table %s\n", tableName)
	if _, err := s.Conn.ExecContext(ctx, query); err != nil {
		return err
	}
	return nil
}

func (w *ddlManager) appendPartition(query string, partKeys string) string {
	if w.parts <= 1 {
		return query
	}
	if w.partitionType == PartitionTypeListAsHash {
		// Generate LIST partitions equivalent with HASH partitions
		s := fmt.Sprintf("%s\nPARTITION BY LIST (%s)\n(", query, partKeys)
		for i := 0; i < w.parts; i++ {
			if i > 0 {
				s = s + ",\n "
			}
			var part string
			for j := i; j < w.warehouses; j = j + w.parts {
				if j > i {
					part = part + ","
				}
				part = part + fmt.Sprintf("%d", j+1)
			}
			s = fmt.Sprintf("%sPARTITION p%d VALUES IN (%s)", s, i, part)
		}
		return s + ")"
	} else if w.partitionType == PartitionTypeListAsRange {
		// Generate LIST partitions equivalent with RANGE partitions
		s := fmt.Sprintf("%s\nPARTITION BY LIST (%s)\n(", query, partKeys)
		addedWarehouses := 0
		for i := 0; i < w.parts; i++ {
			if i > 0 {
				s = s + ",\n "
			}
			warehousesToAdd := w.warehouses - addedWarehouses
			partsLeft := w.parts - i
			warehousesPerPartition := warehousesToAdd / partsLeft
			if (warehousesToAdd % partsLeft) != 0 {
				warehousesPerPartition++
			}
			var part string
			for j := 0; j < warehousesPerPartition; j++ {
				if j > 0 {
					part = part + ","
				}
				addedWarehouses++
				part = part + fmt.Sprintf("%d", addedWarehouses)
			}
			s = fmt.Sprintf("%sPARTITION p%d VALUES IN (%s)", s, i, part)
		}
		return s + ")"
	} else if w.partitionType == PartitionTypeRange {
		// Generate RANGE partitions
		s := fmt.Sprintf("%s\nPARTITION BY RANGE (%s)\n(", query, partKeys)
		for i := 0; i < w.parts; i++ {
			if i > 0 {
				s = s + ",\n "
			}
			warehousesPerPartition := w.warehouses / w.parts
			if (w.warehouses % w.parts) != 0 {
				warehousesPerPartition++
			}
			s = fmt.Sprintf("%sPARTITION p%d VALUES LESS THAN (%d)", s, i, 1+(i+1)*warehousesPerPartition)
		}
		return s + ")"
	}

	return fmt.Sprintf("%s\nPARTITION BY HASH(%s)\nPARTITIONS %d", query, partKeys, w.parts)
}

// createTables creates tables schema.
func (w *ddlManager) createTables(ctx context.Context) error {
	// Warehouse
	query := `
CREATE TABLE IF NOT EXISTS warehouse (
	w_id INT NOT NULL,
	w_name VARCHAR(10),
	w_street_1 VARCHAR(20),
	w_street_2 VARCHAR(20),
	w_city VARCHAR(20),
	w_state CHAR(2),
	w_zip CHAR(9),
	w_tax DECIMAL(4, 4),
	w_ytd DECIMAL(12, 2),
	PRIMARY KEY (w_id)
)`

	query = w.appendPartition(query, "w_id")

	if err := w.createTableDDL(ctx, query, tableWareHouse); err != nil {
		return err
	}

	// District
	query = `
CREATE TABLE IF NOT EXISTS district (
	d_id INT NOT NULL,
	d_w_id INT NOT NULL,
	d_name VARCHAR(10),
	d_street_1 VARCHAR(20),
	d_street_2 VARCHAR(20),
	d_city VARCHAR(20),
	d_state CHAR(2),
	d_zip CHAR(9),
	d_tax DECIMAL(4, 4),
	d_ytd DECIMAL(12, 2),
	d_next_o_id INT,
	PRIMARY KEY (d_w_id, d_id)
)`

	query = w.appendPartition(query, "d_w_id")

	if err := w.createTableDDL(ctx, query, tableDistrict); err != nil {
		return err
	}

	// Customer
	query = `
CREATE TABLE IF NOT EXISTS customer (
	c_id INT NOT NULL, 
	c_d_id INT NOT NULL,
	c_w_id INT NOT NULL, 
	c_first VARCHAR(16), 
	c_middle CHAR(2), 
	c_last VARCHAR(16), 
	c_street_1 VARCHAR(20), 
	c_street_2 VARCHAR(20), 
	c_city VARCHAR(20), 
	c_state CHAR(2), 
	c_zip CHAR(9), 
	c_phone CHAR(16), 
	c_since DATETIME, 
	c_credit CHAR(2), 
	c_credit_lim DECIMAL(12, 2), 
	c_discount DECIMAL(4,4), 
	c_balance DECIMAL(12,2), 
	c_ytd_payment DECIMAL(12,2), 
	c_payment_cnt INT, 
	c_delivery_cnt INT, 
	c_data VARCHAR(500),
	PRIMARY KEY(c_w_id, c_d_id, c_id),
	INDEX idx_customer (c_w_id, c_d_id, c_last, c_first)
)`

	query = w.appendPartition(query, "c_w_id")

	if err := w.createTableDDL(ctx, query, tableCustomer); err != nil {
		return err
	}

	query = `
CREATE TABLE IF NOT EXISTS history (
	h_c_id INT NOT NULL,
	h_c_d_id INT NOT NULL,
	h_c_w_id INT NOT NULL,
	h_d_id INT NOT NULL,
	h_w_id INT NOT NULL,
	h_date DATETIME,
	h_amount DECIMAL(6, 2),
	h_data VARCHAR(24),
	INDEX idx_h_w_id (h_w_id),
	INDEX idx_h_c_w_id (h_c_w_id),
	id int not null AUTO_INCREMENT PRIMARY KEY
)`

	query = w.appendPartition(query, "h_w_id")

	if err := w.createTableDDL(ctx, query, tableHistory); err != nil {
		return err
	}

	query = `
CREATE TABLE IF NOT EXISTS new_order (
	no_o_id INT NOT NULL,
	no_d_id INT NOT NULL,
	no_w_id INT NOT NULL,
	PRIMARY KEY(no_w_id, no_d_id, no_o_id)
)`

	query = w.appendPartition(query, "no_w_id")
	if err := w.createTableDDL(ctx, query, tableNewOrder); err != nil {
		return err
	}

	// because order is a keyword, so here we use orders instead
	query = `
CREATE TABLE IF NOT EXISTS orders (
	o_id INT NOT NULL,
	o_d_id INT NOT NULL,
	o_w_id INT NOT NULL,
	o_c_id INT,
	o_entry_d DATETIME,
	o_carrier_id INT,
	o_ol_cnt INT,
	o_all_local INT,
	PRIMARY KEY(o_w_id, o_d_id, o_id),
	INDEX idx_order (o_w_id, o_d_id, o_c_id, o_id)
)`

	query = w.appendPartition(query, "o_w_id")
	if err := w.createTableDDL(ctx, query, tableOrders); err != nil {
		return err
	}

	query = `
	CREATE TABLE IF NOT EXISTS order_line (
		ol_o_id INT NOT NULL,
		ol_d_id INT NOT NULL,
		ol_w_id INT NOT NULL,
		ol_number INT NOT NULL,
		ol_i_id INT NOT NULL,
		ol_supply_w_id INT,
		ol_delivery_d DATETIME,
		ol_quantity INT,
		ol_amount DECIMAL(6, 2),
		ol_dist_info CHAR(24),
		PRIMARY KEY(ol_w_id, ol_d_id, ol_o_id, ol_number)
)`

	query = w.appendPartition(query, "ol_w_id")
	if err := w.createTableDDL(ctx, query, tableOrderLine); err != nil {
		return err
	}

	query = `
CREATE TABLE IF NOT EXISTS stock (
	s_i_id INT NOT NULL,
	s_w_id INT NOT NULL,
	s_quantity INT,
	s_dist_01 CHAR(24), 
	s_dist_02 CHAR(24),
	s_dist_03 CHAR(24),
	s_dist_04 CHAR(24), 
	s_dist_05 CHAR(24), 
	s_dist_06 CHAR(24), 
	s_dist_07 CHAR(24), 
	s_dist_08 CHAR(24), 
	s_dist_09 CHAR(24), 
	s_dist_10 CHAR(24), 
	s_ytd INT, 
	s_order_cnt INT, 
	s_remote_cnt INT,
	s_data VARCHAR(50),
	PRIMARY KEY(s_w_id, s_i_id)
)`

	query = w.appendPartition(query, "s_w_id")
	if err := w.createTableDDL(ctx, query, tableStock); err != nil {
		return err
	}

	query = `
CREATE TABLE IF NOT EXISTS item (
	i_id INT NOT NULL,
	i_im_id INT,
	i_name VARCHAR(24),
	i_price DECIMAL(5, 2),
	i_data VARCHAR(50),
	PRIMARY KEY(i_id)
)`

	if err := w.createTableDDL(ctx, query, tableItem); err != nil {
		return err
	}

	if w.useFK {
		// TODO: Add foreign key constraint
	}

	if w.parts > 1 {
		// TODO: add PARTITION

	}

	return nil
}

func (w *ddlManager) dropTable(ctx context.Context) error {
	s := getTPCCState(ctx)
	for _, tbl := range tables {
		fmt.Printf("DROP TABLE IF EXISTS %s\n", tbl)
		if _, err := s.Conn.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tbl)); err != nil {
			return err
		}
	}

	return nil
}
