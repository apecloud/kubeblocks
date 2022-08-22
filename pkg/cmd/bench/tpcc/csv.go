package tpcc

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pingcap/go-tpc/pkg/load"
	"github.com/pingcap/go-tpc/pkg/util"
	"github.com/pingcap/go-tpc/pkg/workload"
)

type CSVWorkLoader struct {
	db  *sql.DB
	cfg *Config

	// tables is a set contains which table needs
	// to be generated when preparing csv data.
	tables map[string]bool

	createTableWg sync.WaitGroup
	initLoadTime  string

	ddlManager *ddlManager
}

// NewCSVWorkloader creates the tpc-c workloader to generate CSV files
func NewCSVWorkloader(db *sql.DB, cfg *Config) (*CSVWorkLoader, error) {
	if cfg.Parts > cfg.Warehouses {
		panic(fmt.Errorf("number warehouses %d must >= partition %d", cfg.Warehouses, cfg.Parts))
	}

	w := &CSVWorkLoader{
		db:           db,
		cfg:          cfg,
		initLoadTime: time.Now().Format(timeFormat),
		tables:       make(map[string]bool),
		ddlManager:   newDDLManager(cfg.Parts, cfg.UseFK, cfg.Warehouses, cfg.PartitionType),
	}

	var val bool
	if len(cfg.SpecifiedTables) == 0 {
		val = true
	}
	for _, table := range tables {
		w.tables[table] = val
	}

	if _, err := os.Stat(w.cfg.OutputDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(w.cfg.OutputDir, os.ModePerm); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if len(cfg.SpecifiedTables) > 0 {
		for _, t := range strings.Split(cfg.SpecifiedTables, ",") {
			if _, ok := w.tables[t]; !ok {
				return nil, fmt.Errorf("\nTable %s is not supported.\nSupported tables: item, customer, district, "+
					"orders, new_order, order_line, history, warehouse, stock", t)
			}
			w.tables[t] = true
		}
	}

	if !w.tables[tableOrders] && w.tables[tableOrderLine] {
		return nil, fmt.Errorf("\nTable orders must be specified if you want to generate table order_line")
	}
	if w.db != nil {
		w.createTableWg.Add(cfg.Threads)
	}

	return w, nil
}

func (c *CSVWorkLoader) Name() string {
	return "tpcc-csv"
}

func (c *CSVWorkLoader) InitThread(ctx context.Context, threadID int) context.Context {
	s := &tpccState{
		TpcState: workload.NewTpcState(ctx, c.db),
	}

	s.loaders = make(map[string]*load.CSVBatchLoader)
	for k, v := range c.tables {
		// table item only created at thread 0
		if v && !(k == "item" && threadID != 0) {
			file := util.CreateFile(path.Join(c.cfg.OutputDir, fmt.Sprintf("%s.%s.%d.csv", c.DBName(), k, threadID)))
			s.loaders[k] = load.NewCSVBatchLoader(file)
		}
	}

	ctx = context.WithValue(ctx, stateKey, s)
	return ctx
}

func (c *CSVWorkLoader) CleanupThread(ctx context.Context, _ int) {
	s := getTPCCState(ctx)
	if s.Conn != nil {
		s.Conn.Close()
	}
	for k := range s.loaders {
		s.loaders[k].Close(ctx)
	}
}

func (c *CSVWorkLoader) Prepare(ctx context.Context, threadID int) error {
	if c.db != nil {
		if threadID == 0 {
			if err := c.ddlManager.createTables(ctx); err != nil {
				return err
			}
		}
		c.createTableWg.Done()
		c.createTableWg.Wait()
	}

	return prepareWorkload(ctx, c, c.cfg.Threads, c.cfg.Warehouses, threadID)
}

// CheckPrepare CSV type doesn't support CheckPrepare
func (c *CSVWorkLoader) CheckPrepare(_ context.Context, _ int) error {
	return nil
}

// Run CSV type doesn't support Run
func (c *CSVWorkLoader) Run(_ context.Context, _ int) error {
	return nil
}

// Cleanup CSV type doesn't support Cleanup
func (c *CSVWorkLoader) Cleanup(_ context.Context, _ int) error {
	return nil
}

// Check CSV type doesn't support Check
func (c *CSVWorkLoader) Check(_ context.Context, _ int) error {
	return nil
}

// OutputStats just do nothing
func (c *CSVWorkLoader) OutputStats(_ bool) {}

func (c *CSVWorkLoader) DBName() string {
	return c.cfg.DBName
}

func (c *CSVWorkLoader) loadItem(ctx context.Context) error {
	if !c.tables[tableItem] {
		return nil
	}
	fmt.Printf("load to item\n")
	s := getTPCCState(ctx)
	l := s.loaders[tableItem]

	for i := 0; i < maxItems; i++ {
		s.Buf.Reset()

		iImID := randInt(s.R, 1, 10000)
		iPrice := float64(randInt(s.R, 100, 10000)) / 100.0
		iName := randChars(s.R, s.Buf, 14, 24)
		iData := randOriginalString(s.R, s.Buf)

		v := []string{strconv.Itoa(i + 1), strconv.Itoa(iImID), iName, fmt.Sprintf("%f", iPrice), iData}
		if err := l.InsertValue(ctx, v); err != nil {
			return err
		}
	}

	return l.Flush(ctx)
}

func (c *CSVWorkLoader) loadWarehouse(ctx context.Context, warehouse int) error {
	if !c.tables[tableWareHouse] {
		return nil
	}
	fmt.Printf("load to warehouse in warehouse %d\n", warehouse)
	s := getTPCCState(ctx)
	l := s.loaders[tableWareHouse]

	wName := randChars(s.R, s.Buf, 6, 10)
	wStree1 := randChars(s.R, s.Buf, 10, 20)
	wStree2 := randChars(s.R, s.Buf, 10, 20)
	wCity := randChars(s.R, s.Buf, 10, 20)
	wState := randState(s.R, s.Buf)
	wZip := randZip(s.R, s.Buf)
	wTax := randTax(s.R)
	wYtd := 300000.00

	v := []string{strconv.Itoa(warehouse), wName, wStree1, wStree2, wCity, wState,
		wZip, fmt.Sprintf("%f", wTax), fmt.Sprintf("%f", wYtd)}

	if err := l.InsertValue(ctx, v); err != nil {
		return err
	}

	return l.Flush(ctx)
}

func (c *CSVWorkLoader) loadStock(ctx context.Context, warehouse int) error {
	if !c.tables[tableStock] {
		return nil
	}
	fmt.Printf("load to stock in warehouse %d\n", warehouse)

	s := getTPCCState(ctx)
	l := s.loaders[tableStock]

	for i := 0; i < stockPerWarehouse; i++ {
		s.Buf.Reset()

		sIID := i + 1
		sWID := warehouse
		sQuantity := randInt(s.R, 10, 100)
		sDist01 := randLetters(s.R, s.Buf, 24, 24)
		sDist02 := randLetters(s.R, s.Buf, 24, 24)
		sDist03 := randLetters(s.R, s.Buf, 24, 24)
		sDist04 := randLetters(s.R, s.Buf, 24, 24)
		sDist05 := randLetters(s.R, s.Buf, 24, 24)
		sDist06 := randLetters(s.R, s.Buf, 24, 24)
		sDist07 := randLetters(s.R, s.Buf, 24, 24)
		sDist08 := randLetters(s.R, s.Buf, 24, 24)
		sDist09 := randLetters(s.R, s.Buf, 24, 24)
		sDist10 := randLetters(s.R, s.Buf, 24, 24)
		sYtd := 0
		sOrderCnt := 0
		sRemoteCnt := 0
		sData := randOriginalString(s.R, s.Buf)

		v := []string{strconv.Itoa(sIID), strconv.Itoa(sWID), strconv.Itoa(sQuantity), sDist01, sDist02, sDist03, sDist04, sDist05, sDist06,
			sDist07, sDist08, sDist09, sDist10, strconv.Itoa(sYtd), strconv.Itoa(sOrderCnt), strconv.Itoa(sRemoteCnt), sData}

		if err := l.InsertValue(ctx, v); err != nil {
			return err
		}
	}
	return l.Flush(ctx)
}

func (c *CSVWorkLoader) loadDistrict(ctx context.Context, warehouse int) error {
	if !c.tables[tableDistrict] {
		return nil
	}
	fmt.Printf("load to district in warehouse %d\n", warehouse)

	s := getTPCCState(ctx)
	l := s.loaders[tableDistrict]

	for i := 0; i < districtPerWarehouse; i++ {
		s.Buf.Reset()

		dID := i + 1
		dWID := warehouse
		dName := randChars(s.R, s.Buf, 6, 10)
		dStreet1 := randChars(s.R, s.Buf, 10, 20)
		dStreet2 := randChars(s.R, s.Buf, 10, 20)
		dCity := randChars(s.R, s.Buf, 10, 20)
		dState := randState(s.R, s.Buf)
		dZip := randZip(s.R, s.Buf)
		dTax := randTax(s.R)
		dYtd := 30000.00
		dNextOID := 3001

		v := []string{strconv.Itoa(dID), strconv.Itoa(dWID), dName, dStreet1, dStreet2, dCity, dState, dZip,
			fmt.Sprintf("%f", dTax), fmt.Sprintf("%f", dYtd), strconv.Itoa(dNextOID)}

		if err := l.InsertValue(ctx, v); err != nil {
			return err
		}
	}
	return l.Flush(ctx)
}

func (c *CSVWorkLoader) loadCustomer(ctx context.Context, warehouse int, district int) error {
	if !c.tables[tableCustomer] {
		return nil
	}
	fmt.Printf("load to customer in warehouse %d district %d\n", warehouse, district)

	s := getTPCCState(ctx)
	l := s.loaders[tableCustomer]

	for i := 0; i < customerPerDistrict; i++ {
		s.Buf.Reset()

		cID := i + 1
		cDID := district
		cWID := warehouse
		var cLast string
		if i < 1000 {
			cLast = randCLastSyllables(i, s.Buf)
		} else {
			cLast = randCLast(s.R, s.Buf)
		}
		cMiddle := "OE"
		cFirst := randChars(s.R, s.Buf, 8, 16)
		cStreet1 := randChars(s.R, s.Buf, 10, 20)
		cStreet2 := randChars(s.R, s.Buf, 10, 20)
		cCity := randChars(s.R, s.Buf, 10, 20)
		cState := randState(s.R, s.Buf)
		cZip := randZip(s.R, s.Buf)
		cPhone := randNumbers(s.R, s.Buf, 16, 16)
		cSince := c.initLoadTime
		cCredit := "GC"
		if s.R.Intn(10) == 0 {
			cCredit = "BC"
		}
		cCreditLim := 50000.00
		cDisCount := float64(randInt(s.R, 0, 5000)) / float64(10000.0)
		cBalance := -10.00
		cYtdPayment := 10.00
		cPaymentCnt := 1
		cDeliveryCnt := 0
		cData := randChars(s.R, s.Buf, 300, 500)

		v := []string{strconv.Itoa(cID), strconv.Itoa(cDID), strconv.Itoa(cWID), cFirst, cMiddle, cLast, cStreet1, cStreet2, cCity, cState,
			cZip, cPhone, cSince, cCredit, fmt.Sprintf("%f", cCreditLim), fmt.Sprintf("%f", cDisCount),
			fmt.Sprintf("%f", cBalance), fmt.Sprintf("%f", cYtdPayment), strconv.Itoa(cPaymentCnt), strconv.Itoa(cDeliveryCnt), cData}

		if err := l.InsertValue(ctx, v); err != nil {
			return err
		}
	}

	return l.Flush(ctx)
}

func (c *CSVWorkLoader) loadHistory(ctx context.Context, warehouse int, district int) error {
	if !c.tables[tableHistory] {
		return nil
	}
	fmt.Printf("load to history in warehouse %d district %d\n", warehouse, district)

	s := getTPCCState(ctx)
	l := s.loaders[tableHistory]

	// 1 customer has 1 row
	for i := 0; i < customerPerDistrict; i++ {
		s.Buf.Reset()

		hCID := i + 1
		hCDID := district
		hCWID := warehouse
		hDID := district
		hWID := warehouse
		hDate := c.initLoadTime
		hAmount := 10.00
		hData := randChars(s.R, s.Buf, 12, 24)

		v := []string{strconv.Itoa(hCID), strconv.Itoa(hCDID), strconv.Itoa(hCWID), strconv.Itoa(hDID),
			strconv.Itoa(hWID), hDate, fmt.Sprintf("%f", hAmount), hData}

		if err := l.InsertValue(ctx, v); err != nil {
			return err
		}
	}
	return l.Flush(ctx)
}

func (c *CSVWorkLoader) loadOrder(ctx context.Context, warehouse int, district int) ([]int, error) {
	if !c.tables[tableOrders] {
		return nil, nil
	}
	fmt.Printf("load to orders in warehouse %d district %d\n", warehouse, district)

	s := getTPCCState(ctx)
	l := s.loaders[tableOrders]

	cids := rand.Perm(orderPerDistrict)
	s.R.Shuffle(len(cids), func(i, j int) {
		cids[i], cids[j] = cids[j], cids[i]
	})
	olCnts := make([]int, orderPerDistrict)
	for i := 0; i < orderPerDistrict; i++ {
		s.Buf.Reset()

		oID := i + 1
		oCID := cids[i] + 1
		oDID := district
		oWID := warehouse
		oEntryD := c.initLoadTime
		oCarrierID := "NULL"
		if oID < 2101 {
			oCarrierID = strconv.FormatInt(int64(randInt(s.R, 1, 10)), 10)
		}
		oOLCnt := randInt(s.R, 5, 15)
		olCnts[i] = oOLCnt
		oAllLocal := 1

		v := []string{strconv.Itoa(oID), strconv.Itoa(oDID), strconv.Itoa(oWID), strconv.Itoa(oCID), oEntryD,
			oCarrierID, strconv.Itoa(oOLCnt), strconv.Itoa(oAllLocal)}
		if err := l.InsertValue(ctx, v); err != nil {
			return nil, err
		}
	}

	return olCnts, l.Flush(ctx)
}

func (c *CSVWorkLoader) loadNewOrder(ctx context.Context, warehouse int, district int) error {
	if !c.tables[tableNewOrder] {
		return nil
	}
	fmt.Printf("load to new_order in warehouse %d district %d\n", warehouse, district)

	s := getTPCCState(ctx)

	l := s.loaders[tableNewOrder]

	for i := 0; i < newOrderPerDistrict; i++ {
		s.Buf.Reset()

		noOID := 2101 + i
		noDID := district
		noWID := warehouse

		v := []string{strconv.Itoa(noOID), strconv.Itoa(noDID), strconv.Itoa(noWID)}

		if err := l.InsertValue(ctx, v); err != nil {
			return err
		}
	}

	return l.Flush(ctx)
}

func (c *CSVWorkLoader) loadOrderLine(ctx context.Context, warehouse int,
	district int, olCnts []int) error {
	if !c.tables[tableOrderLine] {
		return nil
	}
	fmt.Printf("load to order_line in warehouse %d district %d\n", warehouse, district)

	s := getTPCCState(ctx)

	l := s.loaders[tableOrderLine]

	for i := 0; i < orderPerDistrict; i++ {
		for j := 0; j < olCnts[i]; j++ {
			s.Buf.Reset()

			olOID := i + 1
			olDID := district
			olWID := warehouse
			olNumber := j + 1
			olIID := randInt(s.R, 1, 100000)
			olSupplyWID := warehouse
			olQuantity := 5

			var olAmount float64
			var olDeliveryD string
			if olOID < 2101 {
				olDeliveryD = c.initLoadTime
				olAmount = 0.00
			} else {
				olDeliveryD = "NULL"
				olAmount = float64(randInt(s.R, 1, 999999)) / 100.0
			}
			olDistInfo := randChars(s.R, s.Buf, 24, 24)

			v := []string{strconv.Itoa(olOID), strconv.Itoa(olDID), strconv.Itoa(olWID), strconv.Itoa(olNumber), strconv.Itoa(olIID),
				strconv.Itoa(olSupplyWID), olDeliveryD, strconv.Itoa(olQuantity), fmt.Sprintf("%f", olAmount), olDistInfo}

			if err := l.InsertValue(ctx, v); err != nil {
				return err
			}
		}
	}

	return l.Flush(ctx)
}
