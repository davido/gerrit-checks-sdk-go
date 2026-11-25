package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListChecksUsesTypedOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/changes/team%2Frepo~main~I123/revisions/7/checks"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
		}
		if got := r.URL.Query()["o"]; !reflect.DeepEqual(got, []string{"CHECKER"}) {
			t.Errorf("checker query = %v, want [CHECKER]", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, ")]}'\n"+`[{"checker_uuid":"ci:linux","state":"SUCCESSFUL","submit_impact":{"required":false}}]`)
	}))
	defer server.Close()

	checks, err := listChecks(context.Background(), server.URL, "team/repo~main~I123", "7")
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].GetCheckerUuid() != "ci:linux" {
		t.Fatalf("unexpected checks: %+v", checks)
	}
	impact := checks[0].GetSubmitImpact()
	if impact.Required == nil || *impact.Required {
		t.Fatalf("explicit optional requiredness was lost: %+v", impact)
	}
}
