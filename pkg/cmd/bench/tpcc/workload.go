package tpcc

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pingcap/go-tpc/pkg/load"
	"github.com/pingcap/go-tpc/pkg/measurement"
	"github.com/pingcap/go-tpc/pkg/workload"
)

type contextKey string

const stateKey = contextKey("tpcc")

var tables = []string{tableItem, tableCustomer, tableDistrict, tableHistory,
	tableNewOrder, tableOrderLine, tableOrders, tableStock, tableWareHouse}

type txn struct {
	name         string
	action       func(ctx context.Context, threadID int) error
	weight       int
	keyingTime   float64
	thinkingTime float64
}

type tpccState struct {
	*workload.TpcState
	index   int
	decks   []int
	loaders map[string]*load.CSVBatchLoader

	newOrderStmts    map[string]*sql.Stmt
	orderStatusStmts map[string]*sql.Stmt
	deliveryStmts    map[string]*sql.Stmt
	stockLevelStmt   map[string]*sql.Stmt
	paymentStmts     map[string]*sql.Stmt
}

const (
	PartitionTypeHash = iota + 1
	PartitionTypeRange
	PartitionTypeListAsHash
	PartitionTypeListAsRange
)

// Config is the configuration for tpcc workload
type Config struct {
	DBName        string
	Threads       int
	Parts         int
	PartitionType int
	Warehouses    int
	UseFK         bool
	Isolation     int
	CheckAll      bool
	NoCheck       bool
	// Weight for NewOrder, Payment, OrderStatus, Delivery, StockLevel.
	// Should be int between [0, 100) and sums to 100.
	Weight []int

	// whether to involve wait times(keying time&thinking time)
	Wait bool

	MaxMeasureLatency time.Duration

	// for prepare sub-command only
	OutputType      string
	OutputDir       string
	SpecifiedTables string

	// connection, retry count when commiting statement fails, default 0
	PrepareRetryCount    int
	PrepareRetryInterval time.Duration
}

// Workloader is TPCC workload
type Workloader struct {
	db *sql.DB

	cfg *Config

	createTableWg sync.WaitGroup
	initLoadTime  string

	ddlManager *ddlManager

	txns []txn

	// stats
	rtMeasurement       *measurement.Measurement
	waitTimeMeasurement *measurement.Measurement
}

// NewWorkloader creates the tpc-c workloader
func NewWorkloader(db *sql.DB, cfg *Config) (workload.Workloader, error) {
	if db == nil && cfg.OutputType == "" {
		panic(fmt.Errorf("failed to connect to database when loading data"))
	}

	if cfg.Parts > cfg.Warehouses {
		panic(fmt.Errorf("number warehouses %d must >= partition %d", cfg.Warehouses, cfg.Parts))
	}

	if cfg.PartitionType < PartitionTypeHash || cfg.PartitionType > PartitionTypeListAsRange {
		panic(fmt.Errorf("Unknown partition type %d", cfg.PartitionType))
	}
	switch l := len(cfg.Weight); l {
	case 0:
		cfg.Weight = []int{45, 43, 4, 4, 4}
	case 5:
		totalWeight := 0
		for _, w := range cfg.Weight {
			totalWeight += w
		}
		if totalWeight != 100 {
			panic(fmt.Errorf("The sum of weight should be 100: %v", cfg.Weight))
		}
	default:
		panic(fmt.Errorf("Should specify exact 5 weights: %v", cfg.Weight))
	}

	resetMaxLat := func(m *measurement.Measurement) {
		m.MaxLatency = cfg.MaxMeasureLatency
	}

	w := &Workloader{
		db:                  db,
		cfg:                 cfg,
		initLoadTime:        time.Now().Format(timeFormat),
		ddlManager:          newDDLManager(cfg.Parts, cfg.UseFK, cfg.Warehouses, cfg.PartitionType),
		rtMeasurement:       measurement.NewMeasurement(resetMaxLat),
		waitTimeMeasurement: measurement.NewMeasurement(resetMaxLat),
	}

	w.txns = []txn{
		{name: "new_order", action: w.runNewOrder, weight: cfg.Weight[0], keyingTime: 18, thinkingTime: 12},
		{name: "payment", action: w.runPayment, weight: cfg.Weight[1], keyingTime: 3, thinkingTime: 12},
		{name: "order_status", action: w.runOrderStatus, weight: cfg.Weight[2], keyingTime: 2, thinkingTime: 10},
		{name: "delivery", action: w.runDelivery, weight: cfg.Weight[3], keyingTime: 2, thinkingTime: 5},
		{name: "stock_level", action: w.runStockLevel, weight: cfg.Weight[4], keyingTime: 2, thinkingTime: 5},
	}

	if w.db != nil {
		w.createTableWg.Add(cfg.Threads)
	}

	return w, nil
}

// Name implements Workloader interface
func (w *Workloader) Name() string {
	return "tpcc"
}

// InitThread implements Workloader interface
func (w *Workloader) InitThread(ctx context.Context, threadID int) context.Context {
	s := &tpccState{
		TpcState: workload.NewTpcState(ctx, w.db),
		index:    0,
		decks:    make([]int, 0, 23),
	}

	for index, txn := range w.txns {
		for i := 0; i < txn.weight; i++ {
			s.decks = append(s.decks, index)
		}
	}

	s.index = len(s.decks) - 1

	ctx = context.WithValue(ctx, stateKey, s)

	return ctx
}

// CleanupThread implements Workloader interface
func (w *Workloader) CleanupThread(ctx context.Context, threadID int) {
	s := getTPCCState(ctx)
	closeStmts(s.newOrderStmts)
	closeStmts(s.paymentStmts)
	closeStmts(s.deliveryStmts)
	closeStmts(s.stockLevelStmt)
	closeStmts(s.orderStatusStmts)
	// TODO: close stmts for delivery, order status, and stock level
	if s.Conn != nil {
		s.Conn.Close()
	}
	for k := range s.loaders {
		s.loaders[k].Close(ctx)
	}
}

// Prepare implements Workloader interface
func (w *Workloader) Prepare(ctx context.Context, threadID int) error {
	if w.db != nil {
		if threadID == 0 {
			if err := w.ddlManager.createTables(ctx); err != nil {
				return err
			}
		}
		w.createTableWg.Done()
		w.createTableWg.Wait()
	}

	return prepareWorkload(ctx, w, w.cfg.Threads, w.cfg.Warehouses, threadID)
}

func getTPCCState(ctx context.Context) *tpccState {
	s := ctx.Value(stateKey).(*tpccState)
	return s
}

// Run implements Workloader interface
func (w *Workloader) Run(ctx context.Context, threadID int) error {
	s := getTPCCState(ctx)
	refreshConn := false
	if err := s.Conn.PingContext(ctx); err != nil {
		if err := s.RefreshConn(ctx); err != nil {
			return err
		}
		refreshConn = true
	}
	if s.newOrderStmts == nil || refreshConn {
		s.newOrderStmts = map[string]*sql.Stmt{
			newOrderSelectCustomer: prepareStmt(ctx, s.Conn, newOrderSelectCustomer),
			newOrderSelectDistrict: prepareStmt(ctx, s.Conn, newOrderSelectDistrict),
			newOrderUpdateDistrict: prepareStmt(ctx, s.Conn, newOrderUpdateDistrict),
			newOrderInsertOrder:    prepareStmt(ctx, s.Conn, newOrderInsertOrder),
			newOrderInsertNewOrder: prepareStmt(ctx, s.Conn, newOrderInsertNewOrder),
			// batch select items
			// batch select stock for update
			newOrderUpdateStock: prepareStmt(ctx, s.Conn, newOrderUpdateStock),
			// batch insert order_line
		}
		for i := 5; i <= 15; i++ {
			s.newOrderStmts[newOrderSelectItemSQLs[i]] = prepareStmt(ctx, s.Conn, newOrderSelectItemSQLs[i])
			s.newOrderStmts[newOrderSelectStockSQLs[i]] = prepareStmt(ctx, s.Conn, newOrderSelectStockSQLs[i])
			s.newOrderStmts[newOrderInsertOrderLineSQLs[i]] = prepareStmt(ctx, s.Conn, newOrderInsertOrderLineSQLs[i])
		}

		s.paymentStmts = map[string]*sql.Stmt{
			paymentUpdateWarehouse:          prepareStmt(ctx, s.Conn, paymentUpdateWarehouse),
			paymentSelectWarehouse:          prepareStmt(ctx, s.Conn, paymentSelectWarehouse),
			paymentUpdateDistrict:           prepareStmt(ctx, s.Conn, paymentUpdateDistrict),
			paymentSelectDistrict:           prepareStmt(ctx, s.Conn, paymentSelectDistrict),
			paymentSelectCustomerListByLast: prepareStmt(ctx, s.Conn, paymentSelectCustomerListByLast),
			paymentSelectCustomerForUpdate:  prepareStmt(ctx, s.Conn, paymentSelectCustomerForUpdate),
			paymentSelectCustomerData:       prepareStmt(ctx, s.Conn, paymentSelectCustomerData),
			paymentUpdateCustomerWithData:   prepareStmt(ctx, s.Conn, paymentUpdateCustomerWithData),
			paymentUpdateCustomer:           prepareStmt(ctx, s.Conn, paymentUpdateCustomer),
			paymentInsertHistory:            prepareStmt(ctx, s.Conn, paymentInsertHistory),
		}

		s.orderStatusStmts = map[string]*sql.Stmt{
			orderStatusSelectCustomerCntByLast: prepareStmt(ctx, s.Conn, orderStatusSelectCustomerCntByLast),
			orderStatusSelectCustomerByLast:    prepareStmt(ctx, s.Conn, orderStatusSelectCustomerByLast),
			orderStatusSelectCustomerByID:      prepareStmt(ctx, s.Conn, orderStatusSelectCustomerByID),
			orderStatusSelectLatestOrder:       prepareStmt(ctx, s.Conn, orderStatusSelectLatestOrder),
			orderStatusSelectOrderLine:         prepareStmt(ctx, s.Conn, orderStatusSelectOrderLine),
		}
		s.deliveryStmts = map[string]*sql.Stmt{
			deliverySelectNewOrder:  prepareStmt(ctx, s.Conn, deliverySelectNewOrder),
			deliveryDeleteNewOrder:  prepareStmt(ctx, s.Conn, deliveryDeleteNewOrder),
			deliveryUpdateOrder:     prepareStmt(ctx, s.Conn, deliveryUpdateOrder),
			deliverySelectOrders:    prepareStmt(ctx, s.Conn, deliverySelectOrders),
			deliveryUpdateOrderLine: prepareStmt(ctx, s.Conn, deliveryUpdateOrderLine),
			deliverySelectSumAmount: prepareStmt(ctx, s.Conn, deliverySelectSumAmount),
			deliveryUpdateCustomer:  prepareStmt(ctx, s.Conn, deliveryUpdateCustomer),
		}
		s.stockLevelStmt = map[string]*sql.Stmt{
			stockLevelSelectDistrict: prepareStmt(ctx, s.Conn, stockLevelSelectDistrict),
			stockLevelCount:          prepareStmt(ctx, s.Conn, stockLevelCount),
		}
	}

	// refer 5.2.4.2
	if s.index == len(s.decks) {
		s.index = 0
		s.R.Shuffle(len(s.decks), func(i, j int) {
			s.decks[i], s.decks[j] = s.decks[j], s.decks[i]
		})
	}

	txnIndex := s.decks[s.R.Intn(len(s.decks))]
	txn := w.txns[txnIndex]

	// For each transaction type, the Keying Time is constant
	// and must be a minimum of 18 seconds for New Order,
	// 3 seconds for Payment,
	// and 2 seconds each for Order-Status, Delivery, and Stock-Level.
	if w.cfg.Wait {
		start := time.Now()
		time.Sleep(time.Duration(txn.keyingTime * float64(time.Second)))
		w.waitTimeMeasurement.Measure(fmt.Sprintf("keyingTime-%s", txn.name), time.Since(start), nil)
	}

	start := time.Now()
	err := txn.action(ctx, threadID)

	w.rtMeasurement.Measure(txn.name, time.Since(start), err)

	// 5.2.5.4, For each transaction type, think time is taken independently from a negative exponential distribution.
	// Think time, T t , is computed from the following equation: Tt = -log(r) * (mean think time),
	// r = random number uniformly distributed between 0 and 1
	if w.cfg.Wait {
		start := time.Now()
		thinkTime := -math.Log(rand.Float64()) * txn.thinkingTime
		if thinkTime > txn.thinkingTime*10 {
			thinkTime = txn.thinkingTime * 10
		}
		time.Sleep(time.Duration(thinkTime * float64(time.Second)))
		w.waitTimeMeasurement.Measure(fmt.Sprintf("thinkingTime-%s", txn.name), time.Since(start), nil)
	}
	// TODO: add check
	return err
}

// Cleanup implements Workloader interface
func (w *Workloader) Cleanup(ctx context.Context, threadID int) error {
	if threadID == 0 {
		if err := w.ddlManager.dropTable(ctx); err != nil {
			return err
		}
	}
	return nil
}

func outputRtMeasurement(prefix string, opMeasurement map[string]*measurement.Histogram) {
	keys := make([]string, 0, len(opMeasurement))
	for k := range opMeasurement {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, op := range keys {
		hist := opMeasurement[op]
		if !hist.Empty() {
			info := hist.GetInfo()
			op = strings.ToUpper(op)
			elapsedVec.WithLabelValues(op).Set(info.Elapsed)
			sumVec.WithLabelValues(op).Set(info.Sum)
			countVec.WithLabelValues(op).Set(float64(info.Count))
			opsVec.WithLabelValues(op).Set(info.Ops)
			avgVec.WithLabelValues(op).Set(info.Avg)
			p50Vec.WithLabelValues(op).Set(info.P50)
			p90Vec.WithLabelValues(op).Set(info.P90)
			p99Vec.WithLabelValues(op).Set(info.P99)
			p999Vec.WithLabelValues(op).Set(info.P999)
			maxVec.WithLabelValues(op).Set(info.Max)
			fmt.Printf("%s%-6s - %s\n", prefix, op, hist.Summary())
		}
	}
}

func outputWaitTimesMeasurement(prefix string, opMeasurement map[string]*measurement.Histogram) {
	keys := make([]string, len(opMeasurement))
	var i = 0
	for k := range opMeasurement {
		keys[i] = k
		i += 1
	}
	sort.Strings(keys)

	for _, op := range keys {
		hist := opMeasurement[op]
		if !hist.Empty() {
			fmt.Printf("%s%-6s - %.1fs\n", prefix, strings.ToUpper(op), float64(hist.GetInfo().Avg)/1000)
		}
	}
}
func (w *Workloader) OutputStats(ifSummaryReport bool) {
	w.rtMeasurement.Output(ifSummaryReport, outputRtMeasurement)
	if w.cfg.Wait {
		w.waitTimeMeasurement.Output(ifSummaryReport, outputWaitTimesMeasurement)
	}
	if ifSummaryReport {
		hist, e := w.rtMeasurement.OpSumMeasurement["new_order"]
		if e && !hist.Empty() {
			result := hist.GetInfo()
			const specWarehouseFactor = 12.86
			tpmC := result.Ops * 60
			efc := 100 * tpmC / (specWarehouseFactor * float64(w.cfg.Warehouses))
			fmt.Printf("tpmC: %.1f, efficiency: %.1f%%\n", tpmC, efc)
		}
	}
}

// DBName returns the name of test db.
func (w *Workloader) DBName() string {
	return w.cfg.DBName
}

func (w *Workloader) beginTx(ctx context.Context) (*sql.Tx, error) {
	s := getTPCCState(ctx)
	tx, err := s.Conn.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.IsolationLevel(w.cfg.Isolation),
	})
	return tx, err
}

//func prepareStmts(ctx context.Context, conn *sql.Conn, queries []string) []*sql.Stmt {
//	stmts := make([]*sql.Stmt, len(queries))
//	for i, query := range queries {
//		if len(query) == 0 {
//			continue
//		}
//		stmts[i] = prepareStmt(ctx, conn, query)
//	}
//
//	return stmts
//}

func prepareStmt(ctx context.Context, conn *sql.Conn, query string) *sql.Stmt {
	stmt, err := conn.PrepareContext(ctx, query)
	if err != nil {
		panic(err)
	}
	return stmt
}

func closeStmts(stmts map[string]*sql.Stmt) {
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		stmt.Close()
	}
}
