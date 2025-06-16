package main

import (
	"fmt"
	"reflect"

	"github.com/openfga/go-sdk/client"
)

func main() {
	// Create an empty ClientTupleKey to inspect its structure
	var tupleKey client.ClientTupleKey

	// Use reflection to inspect the fields
	t := reflect.TypeOf(tupleKey)
	fmt.Printf("ClientTupleKey struct fields:\n")
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fmt.Printf("  %s: %s (tag: %s)\n", field.Name, field.Type, field.Tag)
	}

	// Also inspect ClientTupleKeyWithoutCondition
	var tupleKeyWithoutCondition client.ClientTupleKeyWithoutCondition
	t2 := reflect.TypeOf(tupleKeyWithoutCondition)
	fmt.Printf("\nClientTupleKeyWithoutCondition struct fields:\n")
	for i := 0; i < t2.NumField(); i++ {
		field := t2.Field(i)
		fmt.Printf("  %s: %s (tag: %s)\n", field.Name, field.Type, field.Tag)
	}
}
