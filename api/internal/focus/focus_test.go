package focus

import (
	"context"
	"slices"
	"testing"
)

func TestCountriesAreNormalizedAndNeverNil(t *testing.T) {
	if c := Countries(context.Background()); c == nil || len(c) != 0 {
		t.Fatalf("empty context = %#v", c)
	}
	ctx := With(context.Background(), []string{"tr", " DE ", "tr", "", "turkey"})
	if got := Countries(ctx); !slices.Equal(got, []string{"TR", "DE"}) {
		t.Fatalf("countries = %v", got)
	}
}
