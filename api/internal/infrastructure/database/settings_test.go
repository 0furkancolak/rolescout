package database

import (
	"context"
	"testing"
)

func TestSettingsStoreAndRemove(t *testing.T) {
	pool := requireDB(t)
	r := NewSettingsRepository(pool)
	ctx := context.Background()
	v := "https://api.example.com/v1"
	if err := r.Apply(ctx, map[string]*string{"llm.base_url": &v, "llm.model": ptr("m")}); err != nil {
		t.Fatal(err)
	}
	all, err := r.All(ctx)
	if err != nil || all["llm.base_url"] != v || all["llm.model"] != "m" {
		t.Fatalf("all = %v err = %v", all, err)
	}
	if err := r.Apply(ctx, map[string]*string{"llm.base_url": nil, "llm.model": ptr("m2")}); err != nil {
		t.Fatal(err)
	}
	all, _ = r.All(ctx)
	if _, ok := all["llm.base_url"]; ok || all["llm.model"] != "m2" {
		t.Fatalf("after remove = %v", all)
	}
	if err := r.Apply(ctx, map[string]*string{"Bad Key": ptr("x")}); err == nil {
		t.Fatal("a malformed key must be refused")
	}
	_ = r.Apply(ctx, map[string]*string{"llm.model": nil})
}

func ptr(s string) *string { return &s }
