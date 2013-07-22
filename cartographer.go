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

type TypeHook func(reflect.Value) error

type Cartographer struct {
  fieldsToColumns map[reflect.Type]map[string]string // Map from an reflect.Type's fields to database columns.
  columnsToFields map[reflect.Type]map[string]string // Map from an reflect.Type's database columns to fields.
  typeCache       map[reflect.Type]bool              // Is the reflect.Type cached?
  structTag       string                             // Struct field tag for field to column mapping.
}

// discoverType returns the type of an object, the type
// of the object pointed to, or an error if the type's kind is not a struct.
func discoverType(object interface{}) (typ reflect.Type, err error) {
  typ = reflect.TypeOf(object)

  if reflect.Ptr == typ.Kind() {
    typ = typ.Elem()
  }

  if reflect.Struct != typ.Kind() {
    return nil, errors.New(fmt.Sprintf("Map expected a struct to be passed in to be replicated and populated, received %T.", object))
  }

  return
}

// cacheType adds an entry to the Cartographer's cache,
// as well as stores the types field's and column's for later
// usage.
func (self *Cartographer) cacheType(typ reflect.Type) {
  self.fieldsToColumns[typ] = make(map[string]string)
  self.columnsToFields[typ] = make(map[string]string)
  self.typeCache[typ] = true

  var numberOfFields = typ.NumField()

  for i := 0; i < numberOfFields; i++ {
    var (
      field       = typ.Field(i)
      fieldName   = field.Name
      fieldColumn = field.Tag.Get(self.GetStructTag())
    )

    if 0 != len(fieldColumn) {
      self.columnsToFields[typ][fieldColumn] = fieldName
      self.fieldsToColumns[typ][fieldName] = fieldColumn
    }

  }
}

// ColumnsFor returns an array of strings of the types columns if it has been cached.
// If it has not, it attemps to precache the object for later usage, returning
// its column's in an array of strings after the caching is completed.
func (self *Cartographer) ColumnsFor(object interface{}) (columns []string, err error) {
  typ, err := discoverType(object)

  if nil != err {
    return
  }

  if _, cached := self.typeCache[typ]; !cached {
    self.cacheType(typ)
  }

  for key, _ := range self.columnsToFields[typ] {
    columns = append(columns, key)
  }

  return
}

// FieldsFor returns an array of strings of the type's fields if it has been cached.
// If it has not, it attemps to precache the object for later usage, returning
// its field's in an array of strings after the caching is completed.
func (self *Cartographer) FieldsFor(object interface{}) (fields []string, err error) {
  typ, err := discoverType(object)

  if nil != err {
    return
  }

  if _, cached := self.typeCache[typ]; !cached {
    self.cacheType(typ)
  }

  for key, _ := range self.fieldsToColumns[typ] {
    fields = append(fields, key)
  }

  return
}

// SetStructTag sets the struct tag string that maps struct fields
// to their database column's.
func (self *Cartographer) SetStructTag(tag string) {
  self.structTag = tag
}

// GetStructTag returns the struct tag string that maps struct fields
// to their database column's.
func (self *Cartographer) GetStructTag() string {
  if 0 == len(self.structTag) {
    return "db"
  }

  return self.structTag
}

// GetCachedTypes returns an array of type reflect.Type of types
// that have been cached.
func (self *Cartographer) GetCachedTypes() (cache []reflect.Type) {
  for key, _ := range self.typeCache {
    cache = append(cache, key)
  }

  return
}

// Regeister is an attempt to pre-cache an `object`'s
// field names and their `db` tags intended to be a map
// to corresponding database columns, returning an error
// if the type passed is not a struct kind.
func (self *Cartographer) Register(object interface{}) error {
  typ, err := discoverType(object)

  if nil != err {
    return err
  }

  self.cacheType(typ)

  return nil
}

// Map takes any type that implements the ScannableRows interface,
// calling methods Columns, Next, and Scan. Map's parameter `object`
// must have a reflect.Kind of struct. Map attempts to read and
// cache tags on struct fields labled `db`, which is intended to
// be a map to the field's corresponding database column.
// An array of pointers to the `object` struct passed with it's
// members populated based on the names of the columns associated
// with the rows is returned.  Any `hook` passed to map are given
// a replica generated by reflect.New of the `object` parameter.
func (self *Cartographer) Map(rows ScannableRows, object interface{}, hooks ...TypeHook) (results []interface{}, err error) {
  objectType, err := discoverType(object)

  if nil != err {
    return nil, err
  }

  columns, err := rows.Columns()  // Columns returned for the results returned.
  numberOfColumns := len(columns) // Number of columns.

  if nil != err {
    return nil, err
  }

  if _, cached := self.typeCache[objectType]; !cached {
    self.cacheType(objectType)
  }

  for rows.Next() {
    var rowElements = make([]interface{}, numberOfColumns) // Make a buffer array to store the rows values in.

    for index, _ := range rowElements {
      var buffer interface{}
      rowElements[index] = &buffer
    }

    err = rows.Scan(rowElements...)

    if nil != err {
      return nil, err
    }

    // Reflections craziness.
    var (
      objectReplica = reflect.New(objectType) // Create a replica of the same type of `object` passed.
      objectElement = objectReplica.Elem()    // The element the replica points to.
    )

    // Loop over each of the scanned row elements.
    for index, _ := range rowElements {
      var (
        value  = (*rowElements[index].(*interface{}))                                // The dereferenced value at the current index.
        column = columns[index]                                                      // Current column.
        field  = objectElement.FieldByName(self.columnsToFields[objectType][column]) // The field the value belongs to.
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
      if err = hook(objectReplica); nil != err {
        return // TypeHook returned an error, return it to caller to deal with.
      }
    }

    // Finally, append the replica of the passed item.
    results = append(results, objectReplica.Interface())

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
func New() (cartographer *Cartographer) {
  cartographer = new(Cartographer)
  cartographer.fieldsToColumns = make(map[reflect.Type]map[string]string)
  cartographer.columnsToFields = make(map[reflect.Type]map[string]string)
  cartographer.typeCache = make(map[reflect.Type]bool)

  return
}
