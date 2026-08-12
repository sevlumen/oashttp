package oashttp

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func TestGroupConcurrentConfigurationIsRaceFree(t *testing.T) {
	app := New(Config{Info: Info{Title: "Group concurrency", Version: "1.0.0"}})
	group := app.Group("/v1")

	middleware := func(next http.Handler) http.Handler { return next }

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(2)

		go func() {
			defer wg.Done()
			if err := group.Use(middleware); err != nil {
				t.Errorf("Use: %v", err)
			}
		}()

		go func() {
			defer wg.Done()
			child := group.Group(fmt.Sprintf("/g%d", i))
			MapGet(child, "/status", func(context.Context, struct{}) Result[struct{}] {
				return OK(struct{}{})
			}).WithOperationID(fmt.Sprintf("status%d", i)).Produces(http.StatusOK)
		}()
	}

	wg.Wait()

	if _, err := app.Build(); err != nil {
		t.Fatal(err)
	}
}
