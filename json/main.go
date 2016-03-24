package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	var obj interface{}
	bytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(1)
	}
	json.Unmarshal(bytes, &obj)
	b, _ := json.MarshalIndent(obj, "", "   ")
	fmt.Println(string(b))
}
