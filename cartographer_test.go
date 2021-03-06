package cartographer

import (
	"reflect"
	"testing"
)

var instance = Initialize("db")

type scanner struct {
	scanned bool
}

type faker struct {
	Id int `db:"id"`
}

func (self *scanner) Next() bool {
	if !self.scanned {
		self.scanned = true
		return true
	}

	return false
}

func (self *scanner) Columns() ([]string, error) {
	return []string{"id"}, nil
}

func (self *scanner) Scan(dest ...interface{}) error {
	for index, _ := range dest {
		var x = reflect.ValueOf(1).Interface()
		dest[index] = &x
	}

	return nil
}

func TestMap(t *testing.T) {
	results, err := instance.Map(&scanner{}, faker{})

	if nil != err {
		t.Errorf("Basic Map test returned an unexpected error: %v", err)
	}

	if 1 != len(results) {
		t.Errorf("Basic Map test returned unexpected results: %v", results)
	}
}

func TestColumnsFor(t *testing.T) {
	columns, err := instance.ColumnsFor(faker{})

	if nil != err {
		t.Errorf("Basic ColumnsFor test returned an unexpected error: %v", err)
	}

	if 1 != len(columns) || "id" != columns[0] {
		t.Errorf("Basic ColumnsFor test returned unexpected columns: %v", columns)
	}
}

func TestFieldsFor(t *testing.T) {
	fields, err := instance.FieldsFor(faker{})

	if nil != err {
		t.Errorf("Basic FieldsFor test returned an unexpected error: %v", err)
	}

	if 1 != len(fields) || "Id" != fields[0] {
		t.Errorf("Basic FieldsFor test returned unexpected fields: %v", fields)
	}
}

func TestFieldValueMapFor(t *testing.T) {
	values, err := instance.FieldValueMapFor(faker{1})

	if nil != err {
		t.Errorf("Basic FieldValueMapFor test returned an unexpected error: %v", err)
	}

	if 1 != values["Id"] {
		t.Errorf("Basic FieldValueMapFor test returned unexpected map: %v", values)
	}
}

func TestFieldForColumn(t *testing.T) {
	field, err := instance.FieldForColumn(faker{}, "id")

	if nil != err || field != "Id" {
		t.Errorf("Basic FieldForColumn test returned an unexpected results: %v, %v", field, err)
	}

	field, err = instance.FieldForColumn(faker{}, "Id")

	if nil != err || field != "Id" {
		t.Errorf("Basic FieldForColumn test returned an unexpected results: %v, %v", field, err)
	}
}

func TestColumnForField(t *testing.T) {
	column, err := instance.FieldForColumn(faker{}, "id")

	if nil != err || column != "Id" {
		t.Errorf("Basic FieldForColumn test returned an unexpected results: %v, %v", column, err)
	}

	column, err = instance.FieldForColumn(faker{}, "Id")

	if nil != err || column != "Id" {
		t.Errorf("Basic FieldForColumn test returned an unexpected results: %v, %v", column, err)
	}
}
