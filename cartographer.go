package cartographer

import (
  "errors"
  "fmt"
  "reflect"
  "strconv"
)

type ScannableRows interface {
  Next() bool
  Columns() ([]string, error)
  Scan(...interface{}) error
}

type Hook func(reflect.Value) error

type Cartographer struct {
  fieldsToColumns map[reflect.Type]map[string]string // Map from an reflect.Type's fields to database columns.
  columnsToFields map[reflect.Type]map[string]string // Map from an reflect.Type's database columns to fields.
  typeCache       map[reflect.Type]bool              // Is the reflect.Type cached?
  structTag       string                             // Struct field tag for field to column mapping.
}

// DiscoverType the reflect.Type of the `o` parameter passed, caching
// its fields and database columns taken from the fields `db` tag, or an
// error if the reflect.Type's kind is not a struct.
func (self *Cartographer) DiscoverType(o interface{}) (typ reflect.Type, err error) {
  typ = reflect.TypeOf(o)

  if reflect.Ptr == typ.Kind() {
    typ = typ.Elem()
  }

  if reflect.Struct != typ.Kind() {
    err = errors.New(fmt.Sprintf("Expected a struct "+
      " to be passed, received %T.", o))
    return
  }

  if _, cached := self.typeCache[typ]; !cached {
    self.fieldsToColumns[typ] = make(map[string]string)
    self.columnsToFields[typ] = make(map[string]string)
    self.typeCache[typ] = true

    var numberOfFields = typ.NumField()

    for i := 0; i < numberOfFields; i++ {
      var (
        field  = typ.Field(i)
        name   = field.Name
        column = field.Tag.Get(self.structTag)
      )

      if 0 != len(column) {
        self.columnsToFields[typ][column] = name
        self.fieldsToColumns[typ][name] = column
      }

    }
  }

  return
}

// CreateReplica uses the reflect package to create a replica of the interface passed,
// returning a reflect.Value, or an error if `o` is not a struct.
func (self *Cartographer) CreateReplica(o interface{}) (replica reflect.Value, err error) {
  typ, err := self.DiscoverType(o)

  if nil != err {
    return
  }

  replica = reflect.New(typ)
  return
}

// ColumnsFor returns an array of strings of the types columns, or an
// error if `o` is not a struct.
func (self *Cartographer) ColumnsFor(o interface{}) (columns []string, err error) {
  typ, err := self.DiscoverType(o)

  if nil != err {
    return
  }

  for key, _ := range self.columnsToFields[typ] {
    columns = append(columns, key)
  }

  return
}

// FieldsFor returns an array of strings of the types fields, or an
// error if `o` is not a struct.
func (self *Cartographer) FieldsFor(o interface{}) (fields []string, err error) {
  typ, err := self.DiscoverType(o)

  if nil != err {
    return
  }

  for key, _ := range self.fieldsToColumns[typ] {
    fields = append(fields, key)
  }

  return
}

// FieldValueMapFor returns a map of parameter `o`'s fields to their values, or an
// error if `o` is not a struct.
func (self *Cartographer) FieldValueMapFor(o interface{}) (values map[string]interface{}, err error) {
  typ, err := self.DiscoverType(o)

  if nil != err {
    return
  }

  values = make(map[string]interface{})

  item := reflect.ValueOf(o)

  if reflect.Ptr == item.Kind() {
    item = item.Elem()
  }

  for key, _ := range self.fieldsToColumns[typ] {
    values[key] = item.FieldByName(key).Interface()
  }

  return
}

// ModifiedColumnsValuesMapFor accepts a map of strings to interfaces
// intedned to be a snap shot of the object `o` at an early time/previous state,
// returning a map of the column name for the modified field to its value,
// or an error if one occurs.
func (self *Cartographer) ModifiedColumnsValuesMapFor(i map[string]interface{}, o interface{}) (values map[string]interface{}, err error) {
  typ, err := self.DiscoverType(o)
  n, _ := self.FieldValueMapFor(o)

  if nil != err {
    return
  }

  values = make(map[string]interface{})

  for key, value := range n {
    if n[key] != i[key] {
      values[self.fieldsToColumns[typ][key]] = value
    }
  }

  return
}

// Sync is a helper method that is inteded to be used typically after
// an insert statement has been executed and the tables primary key
// that's potentially auto incremented returned.
func (self *Cartographer) Sync(rows ScannableRows, o interface{}, hooks ...Hook) (result interface{}, err error) {
  results, err := self.Map(rows, o, hooks...)

  if nil != err {
    return
  }

  if 1 != len(results) {
    err = errors.New("Sync expected one and only one result to be returned from Map.")
    return
  }

  result = results[0]

  original, err := self.FieldValueMapFor(o)

  if nil != err {
    return
  }

  synced, err := self.FieldValueMapFor(result)

  if nil != err {
    return
  }

  element := reflect.ValueOf(o)

  if reflect.Ptr == element.Kind() {
    element = element.Elem()
  }

  for key, value := range synced {
    zero := reflect.Zero(reflect.TypeOf(value)).Interface()
    if original[key] != synced[key] && value != zero {
      field := element.FieldByName(key)

      if field.CanSet() {
        field.Set(reflect.ValueOf(value))
      } else {
        err = errors.New(fmt.Sprintf("Sync failed to set field %s.", key))
        return
      }
    }
  }

  return o, nil
}

// Map takes any type that implements the ScannableRows interface,
// calling methods Columns, Next, and Scan. Map's parameter `o`
// must have a reflect.Kind of struct. Map attempts to read and
// cache tags on struct fields labled `db`, which is intended to
// be a map to the field's corresponding database column.
// An array of pointers to the `o` struct passed with it's
// members populated based on the names of the columns associated
// with the rows is returned.  Any `hook` passed to map are given
// a replica generated by reflect.New of the `o` parameter,
// a list of it's fields, and their initial values.
func (self *Cartographer) Map(rows ScannableRows, o interface{}, hooks ...Hook) (results []interface{}, err error) {
  columns, err := rows.Columns() // Columns returned for the results returned.

  if nil != err {
    return nil, err
  }

  numberOfColumns := len(columns) // Number of columns.

  for rows.Next() {
    var (
      buffer = generateBuffer(numberOfColumns) // Make a buffer array to store the rows values in.

      replica reflect.Value
      element reflect.Value
    )

    err = rows.Scan(buffer...)

    if nil != err {
      return
    }

    replica, err = self.CreateReplica(o)

    if nil != err {
      return
    }

    element = replica.Elem()

    for index, _ := range buffer {
      var (
        value  = (*buffer[index].(*interface{}))                                   // The dereferenced value at the current index.
        column = columns[index]                                                    // Current column.
        field  = element.FieldByName(self.columnsToFields[element.Type()][column]) // The field the value belongs to.
      )

      if field.CanSet() {
        switch field.Kind() {
        case reflect.String:
          field.SetString(parseString(value))
        case reflect.Int:
          field.SetInt(parseInt(value))
        case reflect.Float32, reflect.Float64:
          field.SetFloat(parseFloat(value))
        case reflect.Bool:
          field.SetBool(parseBool(value))
        case reflect.Struct:
          field.Set(parseStruct(value))
        }
      }
    }

    for _, hook := range hooks {
      if err = hook(replica); nil != err {
        return // Hook returned an error, return it to caller to deal with.
      }
    }

    // Finally, append the replica of the passed item.
    results = append(results, replica.Interface())
  }

  return
}

func generateBuffer(length int) (buffer []interface{}) {
  buffer = make([]interface{}, length)

  for index, _ := range buffer {
    var item interface{}
    buffer[index] = &item
  }

  return
}

func parseString(o interface{}) string {
  return fmt.Sprintf("%s", o)
}

func parseInt(o interface{}) int64 {
  switch o.(type) {
  case int:
    return int64(o.(int))
  case int16:
    return int64(o.(int16))
  case int32:
    return int64(o.(int32))
  default:
    return int64(o.(int64))
  }
}

func parseFloat(o interface{}) float64 {
  switch o.(type) {
  case []uint8:
    // FIXME: Should never error, but still bad pratice.
    float, _ := strconv.ParseFloat(fmt.Sprintf("%s", o), 8)
    return float
  case float32:
    return float64(o.(float32))
  default:
    return float64(o.(float64))
  }
}

func parseBool(o interface{}) bool {
  return o.(bool)
}

func parseStruct(o interface{}) reflect.Value {
  return reflect.ValueOf(o)
}

// New returns a pointer to a new Cartographer type.
func New(structTag string) (cartographer *Cartographer) {
  cartographer = new(Cartographer)
  cartographer.fieldsToColumns = make(map[reflect.Type]map[string]string)
  cartographer.columnsToFields = make(map[reflect.Type]map[string]string)
  cartographer.typeCache = make(map[reflect.Type]bool)
  cartographer.structTag = structTag

  return
}
