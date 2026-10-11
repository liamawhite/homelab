package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/liamawhite/parental/gen/parental/v1"
	"github.com/liamawhite/parental/gen/parental/v1/parentalv1connect"
	"github.com/liamawhite/parental/internal/planner"
	"github.com/liamawhite/parental/internal/storage"
)

func TestHTTPPlanner(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	handler, err := New(planner.New(db))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := parentalv1connect.NewPlannerServiceClient(server.Client(), server.URL, connect.WithProtoJSON())
	ctx := context.Background()
	zero := int32(0)
	if _, err := client.SavePerson(ctx, connect.NewRequest(&v1.SavePersonRequest{Person: &v1.Person{Name: "Zero budget", Color: "#377c65", BudgetDays: &zero}})); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SavePerson(ctx, connect.NewRequest(&v1.SavePersonRequest{Person: &v1.Person{Name: "Unlimited", Color: "#527ec2"}})); err != nil {
		t.Fatal(err)
	}
	result, err := client.GetPlan(ctx, connect.NewRequest(&v1.GetPlanRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Msg.People) != 2 {
		t.Fatal("expected two people")
	}
	for _, p := range result.Msg.People {
		if p.Name == "Zero budget" && (p.BudgetDays == nil || *p.BudgetDays != 0) {
			t.Fatal("zero must survive JSON round trip")
		}
		if p.Name == "Unlimited" && p.BudgetDays != nil {
			t.Fatal("unlimited must remain absent")
		}
	}
	_, err = client.UpdatePlan(ctx, connect.NewRequest(&v1.UpdatePlanRequest{Plan: &v1.Plan{BirthDate: "invalid"}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatal("validation error must survive transport", err)
	}
	for _, path := range []string{"/", "/healthz"} {
		resp, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s returned %d", path, resp.StatusCode)
		}
		if path == "/" && !strings.Contains(strings.ToLower(string(body)), "<!doctype html>") {
			t.Fatal("frontend not embedded; build it before running tests")
		}
	}
}
