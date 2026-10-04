package main

import (
    "encoding/json"
    "os"
    "gopkg.in/yaml.v3"
)

func main() {
    content, err := os.ReadFile(os.Args[1])
    if err != nil { panic(err) }
    var document any
    if err := yaml.Unmarshal(content, &document); err != nil { panic(err) }
    if err := json.NewEncoder(os.Stdout).Encode(document); err != nil { panic(err) }
}
