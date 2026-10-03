package main

import (
	"encoding/json"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	source, err := os.ReadFile("../../docs/api/openapi.yaml")
	if err != nil { panic(err) }
	var document any
	if err := yaml.Unmarshal(source,&document); err != nil { panic(err) }
	output, err := json.MarshalIndent(document,"","  ")
	if err != nil { panic(err) }
	if err := os.WriteFile("internal/http/router/openapi.json",append(output,'\n'),0644); err != nil { panic(err) }
}
