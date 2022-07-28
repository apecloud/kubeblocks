package tpcc

import "testing"

func TestAppendPartition(t *testing.T) {
	ddl := newDDLManager(4, false, 4, PartitionTypeHash)
	s := ddl.appendPartition("<table definition>", "Id")
	expected := `<table definition>
PARTITION BY HASH(Id)
PARTITIONS 4`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(4, false, 4, PartitionTypeRange)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY RANGE (Id)
(PARTITION p0 VALUES LESS THAN (2),
 PARTITION p1 VALUES LESS THAN (3),
 PARTITION p2 VALUES LESS THAN (4),
 PARTITION p3 VALUES LESS THAN (5))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(4, false, 23, PartitionTypeRange)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY RANGE (Id)
(PARTITION p0 VALUES LESS THAN (7),
 PARTITION p1 VALUES LESS THAN (13),
 PARTITION p2 VALUES LESS THAN (19),
 PARTITION p3 VALUES LESS THAN (25))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(4, false, 12, PartitionTypeListAsHash)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY LIST (Id)
(PARTITION p0 VALUES IN (1,5,9),
 PARTITION p1 VALUES IN (2,6,10),
 PARTITION p2 VALUES IN (3,7,11),
 PARTITION p3 VALUES IN (4,8,12))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(3, false, 4, PartitionTypeListAsHash)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY LIST (Id)
(PARTITION p0 VALUES IN (1,4),
 PARTITION p1 VALUES IN (2),
 PARTITION p2 VALUES IN (3))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(4, false, 23, PartitionTypeListAsHash)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY LIST (Id)
(PARTITION p0 VALUES IN (1,5,9,13,17,21),
 PARTITION p1 VALUES IN (2,6,10,14,18,22),
 PARTITION p2 VALUES IN (3,7,11,15,19,23),
 PARTITION p3 VALUES IN (4,8,12,16,20))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(4, false, 12, PartitionTypeListAsRange)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY LIST (Id)
(PARTITION p0 VALUES IN (1,2,3),
 PARTITION p1 VALUES IN (4,5,6),
 PARTITION p2 VALUES IN (7,8,9),
 PARTITION p3 VALUES IN (10,11,12))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(3, false, 4, PartitionTypeListAsRange)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY LIST (Id)
(PARTITION p0 VALUES IN (1,2),
 PARTITION p1 VALUES IN (3),
 PARTITION p2 VALUES IN (4))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}

	ddl = newDDLManager(4, false, 23, PartitionTypeListAsRange)
	s = ddl.appendPartition("<table definition>", "Id")
	expected = `<table definition>
PARTITION BY LIST (Id)
(PARTITION p0 VALUES IN (1,2,3,4,5,6),
 PARTITION p1 VALUES IN (7,8,9,10,11,12),
 PARTITION p2 VALUES IN (13,14,15,16,17,18),
 PARTITION p3 VALUES IN (19,20,21,22,23))`
	if s != expected {
		t.Errorf("got '%s' expected '%s'", s, expected)
	}
}
