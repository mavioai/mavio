//go:build ignore

package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	err := entc.Generate("./schema", &gen.Config{
		Package: "github.com/mavioai/mavio/libs/store/internal/ent",
		Target:  ".",
		Features: []gen.Feature{
			gen.FeatureUpsert,
			gen.FeatureModifier,
			gen.FeatureIntercept,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
