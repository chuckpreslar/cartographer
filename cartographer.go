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

type Cartographer map[reflect.Type]map[string]string

var (
  FieldsToColumns = make(Cartographer)
  ColumnsToFields = make(Cartographer)
  StructTag       = "db"
)

// Map takes any type that implements the Rows interface, returning an
// array of pointers to the object struct passed with it's members populated
// based on the names of the columns associated with the rows.
//
// FIXME: Oh my, refactor this huge method.
func Map(rows Rows, object interface{}) ([]interface{}, error) {
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

  var objectType = reflect.TypeOf(object)

  if objectType.Kind() == reflect.Ptr {
    objectType = objectType.Elem()
  }

  if _, cached := FieldsToColumns[objectType]; !cached {
    FieldsToColumns[objectType] = make(map[string]string)
    ColumnsToFields[objectType] = make(map[string]string)

    var numberOfFields = objectType.NumField()

    for i := 0; i < numberOfFields; i++ {
      var (
        field       = objectType.Field(i)
        fieldName   = field.Name
        fieldColumn = field.Tag.Get(StructTag)
      )

      if 0 == len(fieldColumn) {
        return nil, errors.New(fmt.Sprintf("Could not read tag `%s` for field `%s` on type %s", StructTag, fieldName, objectType))
      }

      ColumnsToFields[objectType][fieldColumn] = fieldName
      FieldsToColumns[objectType][fieldName] = fieldColumn
    }
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
        value  = (*rowElements[index].(*interface{}))
        column = columns[index]
        field  = objectElement.FieldByName(ColumnsToFields[objectType][column])
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
