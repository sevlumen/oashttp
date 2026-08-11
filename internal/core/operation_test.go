package core

import (
	"context"
	"net/http"
	"testing"
)

func TestOperationCarrierStoresOperationAndPrincipal(t *testing.T) {
	base := context.Background()
	if _, ok := OperationFromContext(base); ok {
		t.Fatal("unexpected operation metadata")
	}
	if _, ok := PrincipalFromContext(base); ok {
		t.Fatal("unexpected principal")
	}

	ctx := WithOperationCarrier(base)
	if _, ok := OperationFromContext(ctx); ok {
		t.Fatal("empty carrier must not report an operation")
	}

	info := OperationInfo{ID: "getUser", Method: http.MethodGet, Route: "/users/{id}"}
	returned := WithOperationInfo(ctx, info)
	if returned != ctx {
		t.Fatal("existing carrier should be updated in place")
	}
	got, ok := OperationFromContext(ctx)
	if !ok || got != info {
		t.Fatalf("operation=%#v ok=%v", got, ok)
	}

	principal := &Principal{Subject: "user-1"}
	returned = WithPrincipal(ctx, principal)
	if returned != ctx {
		t.Fatal("existing carrier should retain the same context")
	}
	gotPrincipal, ok := PrincipalFromContext(ctx)
	if !ok || gotPrincipal != principal {
		t.Fatalf("principal=%#v ok=%v", gotPrincipal, ok)
	}

	if WithOperationCarrier(ctx) != ctx {
		t.Fatal("installing a carrier twice should be idempotent")
	}
}

func TestOperationCarrierHelpersWorkWithoutPreinstalledCarrier(t *testing.T) {
	info := OperationInfo{ID: "createUser", Method: http.MethodPost, Route: "/users"}
	operationCtx := WithOperationInfo(context.Background(), info)
	got, ok := OperationFromContext(operationCtx)
	if !ok || got != info {
		t.Fatalf("operation=%#v ok=%v", got, ok)
	}

	principal := &Principal{Subject: "client-1"}
	principalCtx := WithPrincipal(context.Background(), principal)
	gotPrincipal, ok := PrincipalFromContext(principalCtx)
	if !ok || gotPrincipal != principal {
		t.Fatalf("principal=%#v ok=%v", gotPrincipal, ok)
	}
}
