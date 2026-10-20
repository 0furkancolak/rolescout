package domain

import (
	"reflect"
	"testing"
)

func TestFitTechnologies(t *testing.T) {
	fit := FitTechnologies([]string{"node.js", "react", "php", "go", "react"}, []string{"NodeJS", "go"}, []string{"php"})
	if !fit.Known || !reflect.DeepEqual(fit.Matched, []string{"node.js", "go"}) || !reflect.DeepEqual(fit.Missing, []string{"react", "php"}) || !reflect.DeepEqual(fit.Excluded, []string{"php"}) {
		t.Fatalf("fit = %+v", fit)
	}
	if got := FitTechnologies([]string{"go"}, nil, nil); got.Known {
		t.Fatalf("without anything to compare with the fit is unknown: %+v", got)
	}
}
