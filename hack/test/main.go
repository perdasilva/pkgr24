package main

import (
	"fmt"

	"github.com/tidwall/gjson"
)

func main() {
	test := "[1, 2, 3]"
	out := gjson.Parse(test).Array()
	fmt.Println(out)
}
