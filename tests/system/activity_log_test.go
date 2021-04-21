package system

import (
	"context"
	"github.com/cortezaproject/corteza-server/compose/types"
	"github.com/cortezaproject/corteza-server/pkg/activitylog"
	"github.com/cortezaproject/corteza-server/pkg/id"
	"github.com/cortezaproject/corteza-server/store"
	"github.com/cortezaproject/corteza-server/system/service"
	"testing"
)

func (h helper) clearActivityLog() {
	h.noError(store.TruncateActivityLogs(context.Background(), service.DefaultStore))
}

func (h helper) repoMakeActivityLog() *activitylog.Activity {
	var res = &activitylog.Activity{
		ID:             id.Next(),
		ResourceID:     id.Next(),
		ResourceType:   (types.Record{}).LabelResourceKind(),
		ResourceAction: "create",
	}

	h.a.NoError(store.CreateActivityLog(context.Background(), service.DefaultStore, res))

	return res
}

func TestCreateActivityLog(t *testing.T) {
	h := newHelper(t)
	h.clearActionLog()

	h.repoMakeActivityLog()
}
