package cartographer

import (
  "reflect"
  "testing"
)

var instance = New()

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