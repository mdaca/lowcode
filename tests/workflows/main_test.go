package workflows

import (
	"context"
	"fmt"
	"path"
	"testing"

	"github.com/cortezaproject/corteza-server/app"
	"github.com/cortezaproject/corteza-server/automation/service"
	autTypes "github.com/cortezaproject/corteza-server/automation/types"
	"github.com/cortezaproject/corteza-server/pkg/auth"
	"github.com/cortezaproject/corteza-server/pkg/envoy"
	"github.com/cortezaproject/corteza-server/pkg/envoy/csv"
	"github.com/cortezaproject/corteza-server/pkg/envoy/directory"
	"github.com/cortezaproject/corteza-server/pkg/envoy/json"
	envoyStore "github.com/cortezaproject/corteza-server/pkg/envoy/store"
	"github.com/cortezaproject/corteza-server/pkg/envoy/yaml"
	"github.com/cortezaproject/corteza-server/pkg/errors"
	"github.com/cortezaproject/corteza-server/pkg/eventbus"
	"github.com/cortezaproject/corteza-server/pkg/expr"
	"github.com/cortezaproject/corteza-server/pkg/id"
	"github.com/cortezaproject/corteza-server/pkg/logger"
	"github.com/cortezaproject/corteza-server/store"
	sysTypes "github.com/cortezaproject/corteza-server/system/types"
	"github.com/cortezaproject/corteza-server/tests/helpers"
	"github.com/stretchr/testify/require"
)

var (
	defApp   *app.CortezaApp
	defStore store.Storer
	eventBus = eventbus.New()
)

func init() {
	helpers.RecursiveDotEnvLoad()
}

func TestMain(m *testing.M) {
	logger.SetDefault(logger.MakeDebugLogger())
	ctx := context.Background()

	defApp = helpers.NewIntegrationTestApp(ctx, func(app *app.CortezaApp) (err error) {
		// some test suites require action-log enabled
		app.Opt.ActionLog.WorkflowFunctionsEnabled = true
		defStore = app.Store
		eventbus.Set(eventBus)
		return nil
	})

	if err := defApp.Activate(ctx); err != nil {
		panic(fmt.Errorf("could not activate corteza: %v", err))
	}

	m.Run()
}

func cleanup(t *testing.T) {
	var (
		ctx = context.Background()
	)

	if err := defStore.TruncateAutomationWorkflows(ctx); err != nil {
		t.Fatalf("failed to decode scenario data: %v", err)
	}
}

func loadScenario(ctx context.Context, t *testing.T) {
	loadScenarioWithName(ctx, t, "S"+t.Name()[4:])
}

// 1st step in migration to workflow testdata w/o number prefix
//
// When all old scenarios are renamed, replace it with loadScenario.
func loadNewScenario(ctx context.Context, t *testing.T) {
	loadScenarioWithName(ctx, t, t.Name()[5:])
}

func loadScenarioWithName(ctx context.Context, t *testing.T, scenario string) {
	var (
		err error
	)

	cleanup(t)

	decoded, err := directory.Decode(
		ctx,
		path.Join("testdata", scenario),
		yaml.Decoder(),
		csv.Decoder(),
		json.Decoder(),
	)
	if err != nil {
		t.Fatalf("failed to decode scenario data: %v", err)
	}

	storeEnc := envoyStore.NewStoreEncoder(defStore, &envoyStore.EncoderConfig{})

	b := envoy.NewBuilder(storeEnc)
	g, err := b.Build(ctx, decoded...)
	if err != nil {
		t.Fatalf("failed to build structure graph: %v", err)
	}

	if err = envoy.Encode(ctx, g, storeEnc); err != nil {
		t.Fatalf("failed to build structure graph: %v", err)
	}

	// Reload and register workflows
	if err = service.DefaultWorkflow.Load(ctx); err != nil {
		t.Fatalf("failed to reload workflows: %v", err)
	}
}

func bypassRBAC(ctx context.Context) context.Context {
	u := &sysTypes.User{
		ID: id.Next(),
	}

	if err := defStore.CreateUser(ctx, u); err != nil {
		panic(err)
	}

	u.SetRoles(auth.BypassRoles().IDs()...)
	return auth.SetIdentityToContext(ctx, u)
}

func execWorkflow(ctx context.Context, name string, p autTypes.WorkflowExecParams) (*expr.Vars, autTypes.Stacktrace, error) {
	wf, err := defStore.LookupAutomationWorkflowByHandle(ctx, name)
	if err != nil {
		return nil, nil, err
	}

	return service.DefaultWorkflow.Exec(ctx, wf.ID, p)
}

func mustExecWorkflow(ctx context.Context, t *testing.T, name string, p autTypes.WorkflowExecParams) (vars *expr.Vars, strace autTypes.Stacktrace) {
	var err error
	vars, strace, err = execWorkflow(ctx, name, p)
	if err != nil {
		if issues, is := err.(autTypes.WorkflowIssueSet); is {
			for _, i := range issues {
				t.Logf("issue: %s", i.Description)
				t.Logf("       %v", i.Culprit)
			}
		}

		if unw := errors.Unwrap(err); unw != nil {
			err = unw
		}

		t.Fatalf("could not exec %q: %v", name, err)

	}

	return
}

func addRoleMember(ctx context.Context, req *require.Assertions, r string, uu ...string) {
	role, err := store.LookupRoleByHandle(ctx, defStore, r)
	req.NoError(err)

	rr := make([]*sysTypes.RoleMember, len(uu))
	for i, u := range uu {
		usr, err := store.LookupUserByHandle(ctx, defStore, u)
		req.NoError(err)

		rr[i] = &sysTypes.RoleMember{
			RoleID: role.ID,
			UserID: usr.ID,
		}
	}

	req.NoError(store.CreateRoleMember(ctx, defStore, rr...))
}
