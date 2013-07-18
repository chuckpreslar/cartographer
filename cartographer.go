package cartographer

import (
  "errors"
  "fmt"
  "reflect"
)

type Rows interface {
  Next() bool
  Columns() ([]string, error)
  Scan(...interface{}) error
}

type Waypoint map[reflect.Type]map[string]string
type WaypointCache map[reflect.Type]bool

type Cartographer struct {
  fieldsToColumns Waypoint
  columnsToFields Waypoint
  typeCache       WaypointCache
  structTag       string
}

// ColumnsFor returns an array of strings of the types columns if it has been cached.
// If it has not, it attemps to precache the object for later usage, returning
// its column's in an array of strings after the caching is completed.
func (self *Cartographer) ColumnsFor(object interface{}) (columns []string) {
  var typ = discoverType(object)

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
func (self *Cartographer) FieldsFor(object interface{}) (fields []string) {
  var typ = discoverType(object)

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
func (self *Cartographer) SetStructTag(tag string) *Cartographer {
  self.structTag = tag
  return self
}

// GetStructTag returns the struct tag string that maps struct fields
// to their database column's.
func (self *Cartographer) GetStructTag() string {
  if 0 == len(self.structTag) {
    return "db"
  }

  return self.structTag
}

// discoverType returns the type of an object, or the type
// of the object pointed to.
func discoverType(object interface{}) (typ reflect.Type) {
  typ = reflect.TypeOf(object)

  if reflect.Ptr == typ.Kind() {
    typ = typ.Elem()
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

// Map takes any type that implements the Rows interface, returning an
// array of pointers to the object struct passed with it's members populated
// based on the names of the columns associated with the rows.
func (self *Cartographer) Map(rows Rows, object interface{}) ([]interface{}, error) {
  if reflect.Struct != reflect.ValueOf(object).Kind() {
    return nil, errors.New(fmt.Sprintf("Map expected a struct to be passed in to be replicated and populated, received %T.", object))
  }

  var (
    columns, err    = rows.Columns() // Columns returned for the results returned.
    numberOfColumns = len(columns)   // Number of columns.

    results []interface{} // Results to return.
  )

  if nil != err {
    return nil, err
  }

  var objectType = discoverType(object)

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
      objectReplica = reflect.New(objectType)
      objectElement = objectReplica.Elem()
    )

    // Loop over each of the scanned row elements.
    for index, _ := range rowElements {
      var (
        value  = (*rowElements[index].(*interface{}))                                // The dereferenced value at the current index.
        column = columns[index]                                                      // Current column.
        field  = objectElement.FieldByName(self.columnsToFields[objectType][column]) // The field the value belongs to.
      )

      // FIXME: This is just a basic switch for demonstration, needs to be completed.
      if field.CanSet() {
        switch field.Kind() {
        case reflect.String:
          field.SetString(fmt.Sprintf(`%s`, value))
        case reflect.Int:
          field.SetInt(value.(int64))
        }
      }
    }

    // Finally, append the replica of the passed item.
    results = append(results, objectReplica.Interface())

  }

  return results, nil
}

// New creates and returns a pointer to a new Cartographer instance.
func New() (cartographer *Cartographer) {
  cartographer = new(Cartographer)
  cartographer.typeCache = make(WaypointCache)
  cartographer.fieldsToColumns = make(Waypoint)
  cartographer.columnsToFields = make(Waypoint)

  return
}
