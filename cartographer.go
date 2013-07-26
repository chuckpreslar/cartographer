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
func (self *Cartographer) CreateReplica(o interface{}, hooks ...Hook) (replica reflect.Value, err error) {
  typ, err := self.DiscoverType(o)

  if nil != err {
    return
  }

  replica = reflect.New(typ)

  for _, hook := range hooks {
    if err = hook(replica); nil != err {
      return // Hook returned an error, return it to caller to deal with.
    }
  }

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
// that's potentially auto incremented returned, returning the synced
// objected or an error.
func (self *Cartographer) Sync(rows ScannableRows, o interface{}, hooks ...Hook) (err error) {
  typ, err := self.DiscoverType(o)

  if nil != err {
    return
  }

  object := reflect.ValueOf(o)

  if reflect.Ptr != object.Kind() {
    err = errors.New("Sync expected a pointer to be passed for manipulation.")
    return
  }

  element := object.Elem()
  columns, err := rows.Columns()

  if nil != err {
    return
  }

  // FIXME: The logic within this loop is similar enough with Maps's to be refactored into a method.
  for rows.Next() {
    values, err := populatedRowValues(rows, len(columns))

    if nil != err {
      return err
    }

    for index, _ := range values {
      field := element.FieldByName(self.columnsToFields[typ][columns[index]]) // The field the value belongs to.
      err = setFieldValue(field, (*values[index].(*interface{})))

      if nil != err {
        return err
      }
    }

    for _, hook := range hooks {
      if err = hook(object); nil != err {
        return err // Hook returned an error, return it to caller to deal with.
      }
    }
  }

  return
}

// Map takes any type that implements the ScannableRows interface,
// calling methods Columns, Next, and Scan. Map's parameter `o`
// must have a reflect.Kind of struct. Map attempts to read and
// cache tags on struct fields labled with that string passed to the
// Initialize function, which is intended to be a map to the field's
// corresponding database column. An array of pointers to the `o`
// struct passed with it's members populated based on the names
// of the columns associated with the rows is returned.  Any `hook`
// passed to map are given a replica generated by reflect.New of
// the `o` parameter, a list of it's fields, and their initial values.
func (self *Cartographer) Map(rows ScannableRows, o interface{}, hooks ...Hook) (results []interface{}, err error) {
  columns, err := rows.Columns() // Columns returned for the results returned.

  if nil != err {
    return results, err
  }

  // FIXME: The logic within this loop is similar enough with Sync's to be refactored into a method.
  for rows.Next() {
    values, err := populatedRowValues(rows, len(columns))

    if nil != err {
      return results, err
    }

    replica, err := self.CreateReplica(o, hooks...)

    if nil != err {
      return results, err
    }

    element := replica.Elem()

    for index, _ := range values {
      field := element.FieldByName(self.columnsToFields[element.Type()][columns[index]]) // The field the value belongs to.
      err = setFieldValue(field, (*values[index].(*interface{})))

      if nil != err {
        return results, err
      }
    }

    // Finally, append the replica of the passed item.
    results = append(results, replica.Interface())
  }

  return
}

func setFieldValue(field reflect.Value, value interface{}) (err error) {
  if nil == value {
    return
  }

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
  } else {
    err = errors.New("Failed to set field")
  }

  return
}

func populatedRowValues(rows ScannableRows, size int) (values []interface{}, err error) {
  values = generateBuffer(size)
  err = rows.Scan(values...)
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

// Initialize returns a pointer to a new Cartographer type, setting
// its structTag field which it uses to map fields to database
// columns to the one passed as parameter `structTag`.
func Initialize(structTag string) (cartographer *Cartographer) {
  cartographer = new(Cartographer)
  cartographer.fieldsToColumns = make(map[reflect.Type]map[string]string)
  cartographer.columnsToFields = make(map[reflect.Type]map[string]string)
  cartographer.typeCache = make(map[reflect.Type]bool)
  cartographer.structTag = structTag

  return
}
