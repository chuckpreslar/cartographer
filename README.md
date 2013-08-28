# cartographer

Provides mapping to syncing of results from SQL query rows to structs in Go.

[![Build Status](https://drone.io/github.com/chuckpreslar/cartographer/status.png)](https://drone.io/github.com/chuckpreslar/cartographer/latest)

## Installation

With Google's [Go](http://www.golang.org) installed on your machine:

    $ go get -u github.com/chuckpreslar/cartographer

## Usage

A minimalist example:

```go
package main

import (
  "github.com/chuckpreslar/cartographer"
)

type User struct {
  FirstName string `db:"first_name"`
  LastName  string `db:"last_name"`
  Email     string `db:"email"`
}

func main() {
  // Assuming you have a connection to your database and stored it in a variable named `database`...
  rows, err := database.Query(`SELECT "first_name", "last_name", "email" FROM "users"`)


  if nil != err {
    // Handle potential error.
  }

  instance := cartographer.Initialize("db")
  
  users, err := instance.Map(rows, User{})
  
  if nil != err {
    // Handle potential error.
  }
  
  user := users[0].(*User)
  
  // Do stuff!
}
```

## Documentation

View godoc's or visit [godoc.org](http://godoc.org/github.com/chuckpreslar/cartographer).

    $ godoc cartographer
    
## License

> The MIT License (MIT)

> Copyright (c) 2013 Chuck Preslar

> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to deal
> in the Software without restriction, including without limitation the rights
> to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
> copies of the Software, and to permit persons to whom the Software is
> furnished to do so, subject to the following conditions:

> The above copyright notice and this permission notice shall be included in
> all copies or substantial portions of the Software.

> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
> THE SOFTWARE.
