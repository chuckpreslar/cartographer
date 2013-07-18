package cartographer

import (
  "errors"
  "fmt"
  "reflect"
)

type Scannable interface {
  Next() bool
  Columns() ([]string, error)
  Scan(...interface{}) error
}

var (
  fieldsToColumns = make(map[reflect.Type]map[string]string)
  columnsToFields = make(map[reflect.Type]map[string]string)
  typeCache       = make(map[reflect.Type]bool)
  structTag       string
)

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
func cacheType(typ reflect.Type) {
  fieldsToColumns[typ] = make(map[string]string)
  columnsToFields[typ] = make(map[string]string)
  typeCache[typ] = true

  var numberOfFields = typ.NumField()

  for i := 0; i < numberOfFields; i++ {
    var (
      field       = typ.Field(i)
      fieldName   = field.Name
      fieldColumn = field.Tag.Get(GetStructTag())
    )

    if 0 != len(fieldColumn) {
      columnsToFields[typ][fieldColumn] = fieldName
      fieldsToColumns[typ][fieldName] = fieldColumn
    }

  }
}

// ColumnsFor returns an array of strings of the types columns if it has been cached.
// If it has not, it attemps to precache the object for later usage, returning
// its column's in an array of strings after the caching is completed.
func ColumnsFor(object interface{}) (columns []string, err error) {
  typ, err := discoverType(object)

  if nil != err {
    return
  }

  if _, cached := typeCache[typ]; !cached {
    cacheType(typ)
  }

  for key, _ := range columnsToFields[typ] {
    columns = append(columns, key)
  }

  return
}

// FieldsFor returns an array of strings of the type's fields if it has been cached.
// If it has not, it attemps to precache the object for later usage, returning
// its field's in an array of strings after the caching is completed.
func FieldsFor(object interface{}) (fields []string, err error) {
  typ, err := discoverType(object)

  if nil != err {
    return
  }

  if _, cached := typeCache[typ]; !cached {
    cacheType(typ)
  }

  for key, _ := range fieldsToColumns[typ] {
    fields = append(fields, key)
  }

  return
}

// SetStructTag sets the struct tag string that maps struct fields
// to their database column's.
func SetStructTag(tag string) {
  structTag = tag
}

// GetStructTag returns the struct tag string that maps struct fields
// to their database column's.
func GetStructTag() string {
  if 0 == len(structTag) {
    return "db"
  }

  return structTag
}

// GetCachedTypes returns an array of type reflect.Type of types
// that have been cached.
func GetCachedTypes() (cache []reflect.Type) {
  for key, _ := range typeCache {
    cache = append(cache, key)
  }

  return
}

// Regeister is an attempt to pre-cache an object's columns
// and fields, returning an error if the type passed is not a
// struct kind.
func Register(object interface{}) error {
  typ, err := discoverType(object)

  if nil != err {
    return err
  }

  cacheType(typ)

  return nil
}

// Map takes any type that implements the Rows interface, returning an
// array of pointers to the object struct passed with it's members populated
// based on the names of the columns associated with the rows.
func Map(rows Scannable, object interface{}) (results []interface{}, err error) {
  objectType, err := discoverType(object)

  if nil != err {
    return nil, err
  }

  columns, err := rows.Columns()  // Columns returned for the results returned.
  numberOfColumns := len(columns) // Number of columns.

  if nil != err {
    return nil, err
  }

  if _, cached := typeCache[objectType]; !cached {
    cacheType(objectType)
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
        value  = (*rowElements[index].(*interface{}))                           // The dereferenced value at the current index.
        column = columns[index]                                                 // Current column.
        field  = objectElement.FieldByName(columnsToFields[objectType][column]) // The field the value belongs to.
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

  return
}
